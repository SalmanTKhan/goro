package res

import (
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTHORRoundTripFilesystemAndGRFMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patch.thor")
	operations := []PatchOperation{
		{Kind: PatchAdd, Path: "root/data/value.txt", Data: []byte("new-value")},
		{Kind: PatchRemove, Path: "root/data/old.txt"},
	}
	if err := PackTHOR(path, operations, THORPackOptions{UseGRFMerge: true, TargetGRF: "data.grf"}); err != nil {
		t.Fatal(err)
	}
	thor, err := OpenTHOR(path, DeliveryOptions{ExpectedTargetGRF: "data.grf"})
	if err != nil {
		t.Fatal(err)
	}
	defer thor.Close()
	if !thor.UseGRFMerge || thor.TargetGRF != "data.grf" || thor.Mode != 0x30 || thor.Count() != 2 {
		t.Fatalf("THOR metadata = merge=%t target=%q mode=%x count=%d", thor.UseGRFMerge, thor.TargetGRF, thor.Mode, thor.Count())
	}
	data, err := thor.ReadFile("data/value.txt", DeliveryOptions{})
	if err != nil || string(data) != "new-value" {
		t.Fatalf("THOR read = %q, %v", data, err)
	}
	operations, err = thor.Operations(DeliveryOptions{})
	if err != nil || len(operations) != 2 || operations[0].Kind != PatchRemove {
		t.Fatalf("THOR operations = %#v, %v", operations, err)
	}

	overlay := filepath.Join(dir, "overlay")
	_, tombstones, err := ApplyTHOR(path, overlay, DeliveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tombstones) != 1 || string(mustRead(t, filepath.Join(overlay, "data", "value.txt"))) != "new-value" {
		t.Fatalf("THOR overlay tombstones=%v", tombstones)
	}
}

func TestTHORSingleFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single.thor")
	if err := PackTHOR(path, []PatchOperation{{Kind: PatchAdd, Path: "data/one.txt", Data: []byte("single")}}, THORPackOptions{Mode: 0x21}); err != nil {
		t.Fatal(err)
	}
	thor, err := OpenTHOR(path, DefaultDeliveryOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer thor.Close()
	data, err := thor.ReadFile("data/one.txt", DefaultDeliveryOptions())
	if err != nil || string(data) != "single" {
		t.Fatalf("single THOR data=%q err=%v", data, err)
	}
}

func TestRGZRoundTripAndRejectsTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patch.rgz")
	if err := PackRGZ(path, []PatchOperation{{Kind: PatchAdd, Path: "data/nested/value.txt", Data: []byte("rgz")}}); err != nil {
		t.Fatal(err)
	}
	rgz, err := OpenRGZ(path, DefaultDeliveryOptions())
	if err != nil {
		t.Fatal(err)
	}
	operations := rgz.Operations()
	if len(operations) != 1 || operations[0].Path != "data/nested/value.txt" {
		t.Fatalf("RGZ operations=%#v", operations)
	}
	overlay := filepath.Join(t.TempDir(), "overlay")
	if _, err := ApplyRGZ(path, overlay, DefaultDeliveryOptions()); err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, filepath.Join(overlay, "data", "nested", "value.txt"))) != "rgz" {
		t.Fatal("RGZ overlay contents differ")
	}

	bad := filepath.Join(t.TempDir(), "bad.rgz")
	file, err := os.Create(bad)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	_, _ = writer.Write([]byte{'f', 4, 'd', 'a'})
	_ = writer.Close()
	_ = file.Close()
	if _, err := OpenRGZ(bad, DefaultDeliveryOptions()); !errors.Is(err, ErrDeliveryCorrupt) {
		t.Fatalf("truncated RGZ error=%v", err)
	}
}

func TestDeliveryPathNormalizationAndAndroidPolicy(t *testing.T) {
	for _, value := range []string{"/absolute", "../escape", "root/../escape"} {
		if _, err := normalizeDeliveryPath(value); !errors.Is(err, ErrDeliveryUnsafePath) {
			t.Fatalf("path %q error=%v", value, err)
		}
	}
	canonical, err := normalizeDeliveryPath(`ROOT\DATA\Value.TXT`)
	if err != nil || canonical != "data/value.txt" {
		t.Fatalf("normalized path=%q err=%v", canonical, err)
	}
	if _, err := validateDeliveryPath("data/client.dll", DeliveryOptions{Android: true}); !errors.Is(err, ErrDeliveryUnsafePath) {
		t.Fatalf("Android executable policy error=%v", err)
	}
}

func TestDirectOverlayNormalizesRootPrefix(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	grfPath := filepath.Join(dir, "root.grf")
	if err := writeTestGRF(grfPath, `root\data\value.txt`, []byte("grf-root")); err != nil {
		t.Fatal(err)
	}
	if err := manager.MountOverlay(AssetOverlay{Name: "grf", Format: DeliveryGRF, Path: grfPath}); err != nil {
		t.Fatal(err)
	}
	data, err := manager.ReadFileExact("data/value.txt")
	if err != nil || string(data) != "grf-root" {
		t.Fatalf("GRF root prefix data=%q err=%v", data, err)
	}

	pakPath := filepath.Join(dir, "root.pak")
	if _, err := PackPAKEntries(pakPath, []PAKPackSource{pakTestSource("root/data/value.txt", []byte("pak-root"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	if err := manager.MountOverlay(AssetOverlay{Name: "pak", Format: DeliveryPAK, Path: pakPath, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	data, err = manager.ReadFileExact("data/value.txt")
	if err != nil || string(data) != "pak-root" {
		t.Fatalf("PAK root prefix data=%q err=%v", data, err)
	}
}

func TestDeliveryValidationAppliesArchiveAndAndroidLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.pak")
	if _, err := PackPAKEntries(path, []PAKPackSource{pakTestSource("data/client.dll", []byte("library"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDeliveryOperations(path, DeliveryPAK, DeliveryOptions{Android: true}); !errors.Is(err, ErrDeliveryUnsafePath) {
		t.Fatalf("Android PAK policy error=%v", err)
	}
	if _, err := ReadDeliveryOperations(path, DeliveryPAK, DeliveryOptions{MaxArchiveBytes: 1}); !errors.Is(err, ErrDeliveryCorrupt) {
		t.Fatalf("archive limit error=%v", err)
	}
}

func TestDeliveryAdapterValidatesAllSupportedFormats(t *testing.T) {
	dir := t.TempDir()
	files := map[DeliveryFormat]string{}
	pakPath := filepath.Join(dir, "assets.pak")
	if _, err := PackPAKEntries(pakPath, []PAKPackSource{pakTestSource("data/value.txt", []byte("adapter"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	files[DeliveryPAK] = pakPath
	grfPath := filepath.Join(dir, "assets.grf")
	if err := writeTestGRF(grfPath, "data/value.txt", []byte("adapter")); err != nil {
		t.Fatal(err)
	}
	files[DeliveryGRF] = grfPath
	files[DeliveryGPF] = grfPath
	thorPath := filepath.Join(dir, "assets.thor")
	if err := PackTHOR(thorPath, []PatchOperation{{Kind: PatchAdd, Path: "data/value.txt", Data: []byte("adapter")}}, THORPackOptions{}); err != nil {
		t.Fatal(err)
	}
	files[DeliveryTHOR] = thorPath
	rgzPath := filepath.Join(dir, "assets.rgz")
	if err := PackRGZ(rgzPath, []PatchOperation{{Kind: PatchAdd, Path: "data/value.txt", Data: []byte("adapter")}}); err != nil {
		t.Fatal(err)
	}
	files[DeliveryRGZ] = rgzPath
	for format, file := range files {
		adapter, err := NewDeliveryAdapter(format)
		if err != nil {
			t.Fatalf("adapter %s: %v", format, err)
		}
		if adapter.Format() != format {
			t.Fatalf("adapter format=%s want=%s", adapter.Format(), format)
		}
		operations, err := adapter.Operations(file, DefaultDeliveryOptions())
		if err != nil || len(operations) != 1 || operations[0].Path != "data/value.txt" {
			t.Fatalf("adapter %s operations=%#v err=%v", format, operations, err)
		}
		if err := adapter.Validate(file, DefaultDeliveryOptions()); err != nil {
			t.Fatalf("adapter %s validation: %v", format, err)
		}
	}
}

func TestManagerOverlayPriorityAndTombstones(t *testing.T) {
	dir := t.TempDir()
	if err := writeTestGRF(filepath.Join(dir, "data.grf"), "data/value.txt", []byte("base")); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	patchPath := filepath.Join(dir, "patch.thor")
	if err := PackTHOR(patchPath, []PatchOperation{
		{Kind: PatchAdd, Path: "data/value.txt", Data: []byte("patch")},
		{Kind: PatchAdd, Path: "data/other.txt", Data: []byte("other")},
	}, THORPackOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.MountOverlay(AssetOverlay{Name: "patch", Priority: 10, Format: DeliveryTHOR, Path: patchPath}); err != nil {
		t.Fatal(err)
	}
	if data, err := manager.ReadFile("data/value.txt"); err != nil || string(data) != "patch" {
		t.Fatalf("overlay value=%q err=%v", data, err)
	}
	if data, err := manager.ReadFile("other.txt"); err != nil || string(data) != "other" {
		t.Fatalf("overlay suffix=%q err=%v", data, err)
	}

	deletePath := filepath.Join(dir, "delete.thor")
	if err := PackTHOR(deletePath, []PatchOperation{{Kind: PatchRemove, Path: "data/value.txt"}}, THORPackOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.MountOverlay(AssetOverlay{Name: "delete", Priority: 20, Format: DeliveryTHOR, Path: deletePath}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ReadFileExact("data/value.txt"); err == nil {
		t.Fatal("tombstone did not mask base")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
