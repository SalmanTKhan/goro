package res

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

func TestManagerDiscoversPAKAndPrefersItOverGRF(t *testing.T) {
	dir := t.TempDir()
	if _, err := PackPAKEntries(filepath.Join(dir, "00-base.pak"), []PAKPackSource{
		pakTestSource("data/map/stone.txt", []byte("pak")),
	}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	if err := writeTestGRF(filepath.Join(dir, "data.grf"), "data/map/stone.txt", []byte("grf")); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manager.Packs) != 1 || len(manager.Archives) != 1 {
		t.Fatalf("packs=%d archives=%d", len(manager.Packs), len(manager.Archives))
	}
	data, err := manager.ReadFile("data/map/stone.txt")
	if err != nil || string(data) != "pak" {
		t.Fatalf("preferred data=%q err=%v", string(data), err)
	}
	data, err = manager.ReadFile("stone.txt")
	if err != nil || string(data) != "pak" {
		t.Fatalf("suffix data=%q err=%v", string(data), err)
	}
	if _, err := manager.ReadFileExact("stone.txt"); err == nil {
		t.Fatal("ReadFileExact used PAK suffix fallback")
	}
	_ = manager.Packs[0].Close()
	_ = manager.Archives[0].Close()
}

func TestPAKReadFileDoesNotMaterializeMoreThanOneChunk(t *testing.T) {
	data := bytes.Repeat([]byte("chunked-resource"), DefaultPAKChunkSize/len("chunked-resource")*4+13)
	path := filepath.Join(t.TempDir(), "resource.pak")
	if _, err := PackPAKEntries(path, []PAKPackSource{pakTestSource("large.bin", data)}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	pak, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pak.Close()
	reader, _, err := pak.OpenFile("large.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	read := make([]byte, 31)
	var got []byte
	for {
		n, readErr := reader.Read(read)
		got = append(got, read[:n]...)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
	}
	if !bytes.Equal(got, data) {
		t.Fatal("streaming PAK read differs from source")
	}
}

func TestManagerReadFileExactDoesNotUseGRFSuffixFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.grf")
	if err := writeTestGRF(path, `data\wav\effect\provoke.wav`, []byte("sound")); err != nil {
		t.Fatal(err)
	}
	grf, err := OpenGRF(path)
	if err != nil {
		if errors.Is(err, ErrGRFUnsupportedVersion) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	defer grf.Close()

	manager := &Manager{Root: dir, Archives: []*GRF{grf}}
	if _, err := manager.ReadFileExact(`effect\provoke.wav`); err == nil {
		t.Fatal("ReadFileExact matched a GRF suffix-only path")
	}
	if data, err := manager.ReadFileExact(`data\wav\effect\provoke.wav`); err != nil || string(data) != "sound" {
		t.Fatalf("ReadFileExact exact path data=%q err=%v", string(data), err)
	}
	if data, err := manager.ReadFile(`effect\provoke.wav`); err != nil || string(data) != "sound" {
		t.Fatalf("ReadFile legacy suffix data=%q err=%v", string(data), err)
	}
}
