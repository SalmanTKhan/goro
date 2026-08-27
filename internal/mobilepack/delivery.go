package mobilepack

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kivutar/goro/res"
)

const deliveryManifestFormat = "goro-mobile-delivery"

func buildDelivery(outputDir string, project Project, plan ProjectPlan, artifacts ArtifactManifest, options DeliveryBuildOptions) (*DeliveryManifest, string, string, error) {
	delivery := project.Delivery
	if options.DeliveryEnabled {
		delivery.Enabled = true
	}
	if options.DeliveryBaseURL != "" {
		delivery.BaseURL = options.DeliveryBaseURL
	}
	if options.DeliveryRelease != "" {
		delivery.Release = options.DeliveryRelease
	}
	if options.DeliverySigningKey != "" {
		delivery.SigningKey = options.DeliverySigningKey
	}
	if options.DeliveryPreviousOutput != "" {
		delivery.PreviousOutput = options.DeliveryPreviousOutput
	}
	formats := append([]res.DeliveryFormat(nil), options.DeliveryFormats...)
	if len(formats) == 0 {
		for _, value := range delivery.Formats {
			format, err := res.ParseDeliveryFormat(value)
			if err != nil {
				return nil, "", "", err
			}
			formats = appendUniqueDeliveryFormat(formats, format)
		}
	}
	if !delivery.Enabled && len(formats) == 0 {
		return nil, "", "", nil
	}
	if len(formats) == 0 {
		formats = []res.DeliveryFormat{res.DeliveryPAK}
	}
	if delivery.Release == "" {
		delivery.Release = plan.SourceFingerprint
		if len(delivery.Release) > 16 {
			delivery.Release = delivery.Release[:16]
		}
	}
	if delivery.Release == "" {
		delivery.Release = "local"
	}
	preferred := string(formats[0])
	for _, format := range formats {
		if format == res.DeliveryPAK {
			preferred = string(format)
			break
		}
	}
	manifest := &DeliveryManifest{Format: deliveryManifestFormat, Version: DeliveryManifestVersion, Release: delivery.Release, BaseURL: strings.TrimRight(delivery.BaseURL, "/"), PreferredFormat: preferred}
	if delivery.SigningKey == "" {
		manifest.Warnings = append(manifest.Warnings, "delivery manifest is unsigned; configure an Ed25519 signing key before production use")
	}
	releaseDir := filepath.Join(outputDir, "releases", safePackName(delivery.Release))
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		return nil, "", "", err
	}
	for index, pack := range plan.Packs {
		artifactPack := findArtifactPack(artifacts.Packs, pack.Name)
		if artifactPack == nil {
			return nil, "", "", fmt.Errorf("delivery pack %s is missing from artifact manifest", pack.Name)
		}
		deliveryPack := DeliveryPack{Name: pack.Name, Role: pack.Role, PreferredFormat: preferred, Dependencies: append([]string(nil), pack.Dependencies...), Overrides: append([]string(nil), pack.Overrides...), DownloadPriority: pack.DownloadPriority}
		if len(deliveryPack.Dependencies) == 0 && pack.Role != "base" {
			deliveryPack.Dependencies = []string{plan.Packs[0].Name}
		}
		for _, format := range formats {
			artifact, warning, err := buildDeliveryArtifact(outputDir, releaseDir, plan, pack, *artifactPack, format, delivery.PreviousOutput, index)
			if err != nil {
				return nil, "", "", err
			}
			if warning != "" {
				manifest.Warnings = append(manifest.Warnings, warning)
			}
			if artifact != nil {
				deliveryPack.Artifacts = append(deliveryPack.Artifacts, *artifact)
			}
		}
		if len(deliveryPack.Artifacts) == 0 {
			return nil, "", "", fmt.Errorf("delivery pack %s has no generated artifacts", pack.Name)
		}
		manifest.Packs = append(manifest.Packs, deliveryPack)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, "", "", err
	}
	manifestPath := filepath.Join(outputDir, "mobile-delivery.json")
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
		return nil, "", "", err
	}
	signaturePath := ""
	if delivery.SigningKey != "" {
		privateKey, err := loadDeliverySigningKey(delivery.SigningKey)
		if err != nil {
			return nil, "", "", err
		}
		manifest.PublicKey = base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
		manifest.KeyID = delivery.KeyID
		data, err = json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, "", "", err
		}
		if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
			return nil, "", "", err
		}
		signature := ed25519.Sign(privateKey, append(data, '\n'))
		signaturePath = filepath.Join(outputDir, "mobile-delivery.json.sig")
		if err := os.WriteFile(signaturePath, []byte(base64.StdEncoding.EncodeToString(signature)+"\n"), 0o644); err != nil {
			return nil, "", "", err
		}
	}
	return manifest, manifestPath, signaturePath, nil
}

func buildDeliveryArtifact(outputDir, releaseDir string, plan ProjectPlan, pack PackPlan, existing ArtifactPack, format res.DeliveryFormat, previousOutput string, index int) (*DeliveryArtifact, string, error) {
	if !format.Valid() {
		return nil, "", fmt.Errorf("unsupported delivery format %q", format)
	}
	fileName := fmt.Sprintf("%02d-%s.%s", index, safePackName(pack.Name), format)
	filePath := filepath.Join(releaseDir, fileName)
	relative, _ := filepath.Rel(outputDir, filePath)
	relative = filepath.ToSlash(relative)
	warning := ""
	switch format {
	case res.DeliveryPAK, res.DeliveryGRF:
		if source := findArtifactFile(existing.Files, format); source != nil {
			if err := copyGeneratedFile(filepath.Join(outputDir, filepath.FromSlash(source.File)), filePath); err != nil {
				return nil, "", err
			}
			chunks := 0
			if format == res.DeliveryPAK {
				chunks = artifactPAKChunks(filePath)
			}
			return deliveryFile(filePath, relative, format, 0, chunks)
		} else if format == res.DeliveryPAK {
			stats, err := packPAKAtomic(filePath, pakSources(pack), res.PAKOptions{ChunkSize: res.DefaultPAKChunkSize, ZstdLevel: res.DefaultPAKZstdLevel})
			if err != nil {
				return nil, "", err
			}
			return deliveryFile(filePath, relative, format, stats.ArchiveBytes, stats.Chunks)
		} else {
			stats, err := packGRFAtomic(filePath, grfSources(pack))
			if err != nil {
				return nil, "", err
			}
			return deliveryFile(filePath, relative, format, stats.ArchiveBytes, 0)
		}
	case res.DeliveryGPF:
		if _, err := packGRFAtomic(filePath, grfSources(pack)); err != nil {
			return nil, "", err
		}
	case res.DeliveryTHOR, res.DeliveryRGZ:
		if strings.TrimSpace(previousOutput) == "" {
			return nil, "", fmt.Errorf("delivery format %s requires --delivery-previous-output", format)
		}
		previous, err := previousPackResources(previousOutput, pack.Name)
		if err != nil {
			return nil, "", err
		}
		operations, err := changedOperations(pack, previous)
		if err != nil {
			return nil, "", err
		}
		if format == res.DeliveryTHOR {
			if err := res.PackTHOR(filePath, operations, res.THORPackOptions{}); err != nil {
				return nil, "", err
			}
		} else {
			additions := make([]res.PatchOperation, 0, len(operations))
			for _, operation := range operations {
				if operation.Kind == res.PatchAdd {
					additions = append(additions, operation)
				} else {
					warning = fmt.Sprintf("pack %s RGZ artifact omits %d removals because RGZ has no deletion record", pack.Name, len(operations)-len(additions))
				}
			}
			if err := res.PackRGZ(filePath, additions); err != nil {
				return nil, "", err
			}
		}
	default:
		return nil, "", fmt.Errorf("unsupported delivery format %q", format)
	}
	adapter, err := res.NewDeliveryAdapter(format)
	if err != nil {
		return nil, "", err
	}
	if err := adapter.Validate(filePath, res.DefaultDeliveryOptions()); err != nil {
		return nil, "", fmt.Errorf("validate delivery %s: %w", format, err)
	}
	return deliveryFile(filePath, relative, format, 0, 0, warning)
}

func deliveryFile(filePath, relative string, format res.DeliveryFormat, archiveBytes int64, chunks int, warnings ...string) (*DeliveryArtifact, string, error) {
	hash, size, err := hashFile(filePath)
	if err != nil {
		return nil, "", err
	}
	if archiveBytes == 0 {
		archiveBytes = size
	}
	artifact := &DeliveryArtifact{Format: string(format), URL: relative, SHA256: hash, ArchiveBytes: archiveBytes, Chunks: chunks}
	warning := ""
	if len(warnings) > 0 {
		warning = warnings[0]
	}
	return artifact, warning, nil
}

func appendUniqueDeliveryFormat(formats []res.DeliveryFormat, format res.DeliveryFormat) []res.DeliveryFormat {
	for _, current := range formats {
		if current == format {
			return formats
		}
	}
	return append(formats, format)
}

func findArtifactPack(packs []ArtifactPack, name string) *ArtifactPack {
	for index := range packs {
		if packs[index].Name == name {
			return &packs[index]
		}
	}
	return nil
}

func findArtifactFile(files []ArtifactFile, format res.DeliveryFormat) *ArtifactFile {
	for index := range files {
		if files[index].Format == string(format) {
			return &files[index]
		}
	}
	return nil
}

func artifactPAKChunks(filePath string) int {
	archive, err := res.OpenPAK(filePath)
	if err != nil {
		return 0
	}
	defer archive.Close()
	chunks := 0
	for _, name := range archive.Names() {
		if entry, ok := archive.Entry(name); ok {
			chunks += len(entry.Chunks)
		}
	}
	return chunks
}

func copyGeneratedFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func previousPackResources(outputDir, packName string) (map[string][]byte, error) {
	manifestData, err := os.ReadFile(filepath.Join(outputDir, "mobile-assets.json"))
	if err != nil {
		return nil, fmt.Errorf("read previous mobile asset manifest: %w", err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("decode previous mobile asset manifest: %w", err)
	}
	pack := findArtifactPack(manifest.Packs, packName)
	if pack == nil {
		return map[string][]byte{}, nil
	}
	var source *ArtifactFile
	for _, format := range []res.DeliveryFormat{res.DeliveryPAK, res.DeliveryGRF, res.DeliveryGPF} {
		if candidate := findArtifactFile(pack.Files, format); candidate != nil {
			source = candidate
			break
		}
	}
	if source == nil {
		return map[string][]byte{}, nil
	}
	filePath := filepath.Join(outputDir, filepath.FromSlash(source.File))
	result := map[string][]byte{}
	switch source.Format {
	case string(res.DeliveryPAK):
		archive, err := res.OpenPAK(filePath)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		for _, name := range archive.Names() {
			data, err := archive.ReadFile(name)
			if err != nil {
				return nil, err
			}
			result[name] = data
		}
	case string(res.DeliveryGRF), string(res.DeliveryGPF):
		archive, err := res.OpenGRF(filePath)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		for _, name := range archive.Names() {
			data, err := archive.ReadFile(name)
			if err != nil {
				return nil, err
			}
			result[name] = data
		}
	}
	return result, nil
}

func changedOperations(pack PackPlan, previous map[string][]byte) ([]res.PatchOperation, error) {
	current := make(map[string][]byte, len(pack.Resources))
	for _, resource := range pack.Resources {
		reader, err := resource.open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		current[resource.Name] = data
	}
	paths := make([]string, 0, len(current)+len(previous))
	seen := map[string]struct{}{}
	for name := range current {
		seen[name] = struct{}{}
		paths = append(paths, name)
	}
	for name := range previous {
		if _, exists := seen[name]; !exists {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)
	operations := make([]res.PatchOperation, 0, len(paths))
	for _, name := range paths {
		data, exists := current[name]
		if !exists {
			operations = append(operations, res.PatchOperation{Kind: res.PatchRemove, Path: name})
			continue
		}
		old, wasPresent := previous[name]
		if wasPresent && stringHash(old) == stringHash(data) {
			continue
		}
		operations = append(operations, res.PatchOperation{Kind: res.PatchAdd, Path: name, Data: data, Size: int64(len(data)), SHA256: stringHash(data)})
	}
	return operations, nil
}

func stringHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:])
}

func loadDeliverySigningKey(filePath string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read delivery signing key: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if decoded, decodeErr := hex.DecodeString(trimmed); decodeErr == nil {
		data = decoded
	} else if decoded, decodeErr := base64.StdEncoding.DecodeString(trimmed); decodeErr == nil {
		data = decoded
	}
	switch len(data) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(data), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(data), nil
	default:
		return nil, fmt.Errorf("delivery signing key must be an Ed25519 seed or private key")
	}
}
