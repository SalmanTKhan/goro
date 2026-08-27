package res

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AssetOverlay is one release-local layer. Higher priorities are resolved
// first. Path is a container for PAK/GRF/GPF and a patch artifact for THOR or
// RGZ; patch artifacts are applied into a sibling private overlay directory.
type AssetOverlay struct {
	Name       string         `json:"name"`
	Priority   int            `json:"priority"`
	Format     DeliveryFormat `json:"format"`
	Path       string         `json:"path"`
	Tombstones []string       `json:"tombstones,omitempty"`
	Android    bool           `json:"-"`
}

type mountedOverlay struct {
	config AssetOverlay
	root   string
	pak    *PAK
	grf    *GRF
}

func (m *Manager) MountOverlay(overlay AssetOverlay) error {
	if m == nil {
		return errors.New("nil resource manager")
	}
	if strings.TrimSpace(overlay.Name) == "" {
		return errors.New("overlay name is empty")
	}
	if !overlay.Format.Valid() {
		return fmt.Errorf("%w: %s", ErrDeliveryUnsupportedFormat, overlay.Format)
	}
	if strings.TrimSpace(overlay.Path) == "" {
		return errors.New("overlay path is empty")
	}
	for _, existing := range m.overlays {
		if existing.config.Name == overlay.Name {
			if existing.config.Format == overlay.Format && filepath.Clean(existing.config.Path) == filepath.Clean(overlay.Path) {
				return nil
			}
			return fmt.Errorf("overlay already mounted: %s", overlay.Name)
		}
	}

	mounted := mountedOverlay{config: overlay}
	options := DefaultDeliveryOptions()
	options.Android = overlay.Android
	var err error
	switch overlay.Format {
	case DeliveryPAK:
		if _, err = ReadDeliveryOperations(overlay.Path, overlay.Format, options); err != nil {
			break
		}
		mounted.pak, err = OpenPAK(overlay.Path)
	case DeliveryGRF, DeliveryGPF:
		if _, err = ReadDeliveryOperations(overlay.Path, overlay.Format, options); err != nil {
			break
		}
		mounted.grf, err = OpenGRF(overlay.Path)
	case DeliveryTHOR:
		var tombstones []string
		mounted.root, tombstones, err = m.applyPatchOverlay(overlay, true, options)
		mounted.config.Tombstones = append(mounted.config.Tombstones, tombstones...)
	case DeliveryRGZ:
		mounted.root, _, err = m.applyPatchOverlay(overlay, false, options)
	}
	if err != nil {
		if mounted.pak != nil {
			mounted.pak.Close()
		}
		if mounted.grf != nil {
			mounted.grf.Close()
		}
		return fmt.Errorf("mount overlay %s: %w", overlay.Name, err)
	}
	m.overlays = append(m.overlays, mounted)
	m.Overlays = append(m.Overlays, mounted.config)
	m.sortOverlays()
	return nil
}

func (m *Manager) applyPatchOverlay(overlay AssetOverlay, thor bool, options DeliveryOptions) (string, []string, error) {
	patchInfo, err := os.Stat(overlay.Path)
	if err != nil {
		return "", nil, err
	}
	if patchInfo.IsDir() {
		return filepath.Clean(overlay.Path), append([]string(nil), overlay.Tombstones...), nil
	}
	name := safeOverlayName(overlay.Name)
	root := filepath.Join(filepath.Dir(overlay.Path), ".goro-overlay-"+name)
	if err := os.RemoveAll(root); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", nil, err
	}
	var tombstones []string
	if thor {
		_, tombstones, err = ApplyTHOR(overlay.Path, root, options)
	} else {
		_, err = ApplyRGZ(overlay.Path, root, options)
	}
	if err != nil {
		return "", nil, err
	}
	for _, tombstone := range tombstones {
		if !containsOverlayPath(overlay.Tombstones, tombstone) {
			overlay.Tombstones = append(overlay.Tombstones, tombstone)
		}
	}
	m.sortStrings(overlay.Tombstones)
	return root, overlay.Tombstones, nil
}

func (m *Manager) UnmountOverlay(name string) error {
	for index, overlay := range m.overlays {
		if overlay.config.Name != name {
			continue
		}
		if overlay.pak != nil {
			_ = overlay.pak.Close()
		}
		if overlay.grf != nil {
			_ = overlay.grf.Close()
		}
		m.overlays = append(m.overlays[:index], m.overlays[index+1:]...)
		for publicIndex, publicOverlay := range m.Overlays {
			if publicOverlay.Name == name {
				m.Overlays = append(m.Overlays[:publicIndex], m.Overlays[publicIndex+1:]...)
				break
			}
		}
		return nil
	}
	return fmt.Errorf("overlay not mounted: %s", name)
}

// ReplaceOverlays switches the active release as one manager operation. All
// candidate artifacts are validated by MountOverlay before the new set is
// exposed; if any candidate fails, the previous set is restored.
func (m *Manager) ReplaceOverlays(overlays []AssetOverlay) error {
	if m == nil {
		return errors.New("nil resource manager")
	}
	previous := make([]AssetOverlay, 0, len(m.overlays))
	for _, mounted := range m.overlays {
		previous = append(previous, mounted.config)
	}
	if err := m.unmountAll(); err != nil {
		return err
	}
	for _, overlay := range overlays {
		if err := m.MountOverlay(overlay); err != nil {
			failed := err
			_ = m.unmountAll()
			for _, old := range previous {
				if restoreErr := m.MountOverlay(old); restoreErr != nil {
					return fmt.Errorf("replace overlay %s failed: %w (restore failed: %v)", overlay.Name, failed, restoreErr)
				}
			}
			return fmt.Errorf("replace overlay %s failed: %w", overlay.Name, failed)
		}
	}
	return nil
}

func (m *Manager) unmountAll() error {
	names := make([]string, 0, len(m.overlays))
	for _, overlay := range m.overlays {
		names = append(names, overlay.config.Name)
	}
	var first error
	for _, name := range names {
		first = firstError(first, m.UnmountOverlay(name))
	}
	return first
}

// RefreshOverlays reopens all currently mounted containers and reapplies
// patch artifacts. The operation is all-or-nothing: a failed refresh closes
// the new set only after all prior mounts remain represented in configs.
func (m *Manager) RefreshOverlays() error {
	configs := make([]AssetOverlay, 0, len(m.overlays))
	for _, overlay := range m.overlays {
		configs = append(configs, overlay.config)
	}
	for _, overlay := range m.overlays {
		if overlay.pak != nil {
			_ = overlay.pak.Close()
		}
		if overlay.grf != nil {
			_ = overlay.grf.Close()
		}
	}
	m.overlays = nil
	m.Overlays = nil
	for _, config := range configs {
		if err := m.MountOverlay(config); err != nil {
			// A refresh failure must not leave the manager with a partially
			// activated release. Reconstruct the previous set before returning
			// the validation error.
			failed := err
			m.overlays = nil
			m.Overlays = nil
			for _, previous := range configs {
				if restoreErr := m.MountOverlay(previous); restoreErr != nil {
					return fmt.Errorf("refresh overlay %s failed: %w (restore failed: %v)", config.Name, failed, restoreErr)
				}
			}
			return fmt.Errorf("refresh overlay %s failed: %w", config.Name, failed)
		}
	}
	return nil
}

func (m *Manager) Close() error {
	var first error
	for _, overlay := range m.overlays {
		if overlay.pak != nil {
			first = firstError(first, overlay.pak.Close())
		}
		if overlay.grf != nil {
			first = firstError(first, overlay.grf.Close())
		}
	}
	for _, archive := range m.Archives {
		first = firstError(first, archive.Close())
	}
	for _, pack := range m.Packs {
		first = firstError(first, pack.Close())
	}
	m.overlays = nil
	m.Overlays = nil
	return first
}

func (m *Manager) sortOverlays() {
	sort.SliceStable(m.overlays, func(i, j int) bool {
		if m.overlays[i].config.Priority == m.overlays[j].config.Priority {
			return m.overlays[i].config.Name < m.overlays[j].config.Name
		}
		return m.overlays[i].config.Priority > m.overlays[j].config.Priority
	})
	sort.SliceStable(m.Overlays, func(i, j int) bool {
		if m.Overlays[i].Priority == m.Overlays[j].Priority {
			return m.Overlays[i].Name < m.Overlays[j].Name
		}
		return m.Overlays[i].Priority > m.Overlays[j].Priority
	})
}

func (m *Manager) readOverlay(name string, exact bool) ([]byte, bool, bool, error) {
	lookup := pakLookupName(name)
	if lookup == "" {
		return nil, false, false, ErrDeliveryUnsafePath
	}
	for _, overlay := range m.overlays {
		if overlayTombstones(overlay.config.Tombstones, lookup, exact) {
			return nil, false, true, nil
		}
		if overlay.root != "" {
			if exact {
				if filePath, ok := overlayLooseFile(overlay.root, lookup); ok {
					data, err := os.ReadFile(filePath)
					return data, err == nil, false, err
				}
			} else if filePath, ok := overlayLooseFileWithSuffix(overlay.root, lookup); ok {
				data, err := os.ReadFile(filePath)
				return data, err == nil, false, err
			}
		}
		if overlay.pak != nil {
			data, err := overlay.pak.ReadFile(name)
			if err == nil {
				return data, true, false, nil
			}
			if !exact && errors.Is(err, ErrPAKNotFound) {
				for _, candidate := range overlay.pak.NamesWithSuffix(name) {
					data, readErr := overlay.pak.ReadFile(candidate)
					if readErr == nil {
						return data, true, false, nil
					}
				}
			} else if !errors.Is(err, ErrPAKNotFound) {
				return nil, false, false, err
			}
		}
		if overlay.grf != nil {
			data, err := overlay.grf.ReadFile(name)
			if err == nil {
				return data, true, false, nil
			}
			if !exact && errors.Is(err, ErrGRFNotFound) {
				for _, candidate := range overlay.grf.NamesWithSuffix(name) {
					data, readErr := overlay.grf.ReadFile(candidate)
					if readErr == nil {
						return data, true, false, nil
					}
				}
			} else if !errors.Is(err, ErrGRFNotFound) {
				return nil, false, false, err
			}
		}
	}
	return nil, false, false, nil
}

func overlayLooseFile(root, name string) (string, bool) {
	name = strings.ReplaceAll(name, "/", string(filepath.Separator))
	filePath := filepath.Join(root, name)
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", false
	}
	return filePath, true
}

func overlayLooseFileWithSuffix(root, suffix string) (string, bool) {
	var matches []string
	_ = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err == nil && grfPathSuffixMatch(strings.ToLower(filepath.ToSlash(relative)), suffix) {
			matches = append(matches, filePath)
		}
		return nil
	})
	sort.Strings(matches)
	if len(matches) > 0 {
		return matches[0], true
	}
	return "", false
}

func overlayTombstones(tombstones []string, lookup string, exact bool) bool {
	for _, tombstone := range tombstones {
		canonical, err := normalizeDeliveryPath(tombstone)
		if err != nil {
			continue
		}
		if canonical == lookup || (!exact && grfPathSuffixMatch(canonical, lookup)) {
			return true
		}
	}
	return false
}

func containsOverlayPath(paths []string, name string) bool {
	for _, path := range paths {
		if strings.EqualFold(path, name) {
			return true
		}
	}
	return false
}

func safeOverlayName(name string) string {
	var out strings.Builder
	for _, char := range strings.ToLower(name) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			out.WriteRune(char)
		} else {
			out.WriteByte('-')
		}
	}
	if out.Len() == 0 {
		return "overlay"
	}
	return out.String()
}

func (m *Manager) sortStrings(values []string) { sort.Strings(values) }
