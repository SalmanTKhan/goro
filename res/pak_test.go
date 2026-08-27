package res

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func TestPAKRoundTripUsesIndependentChunksAndRawEntries(t *testing.T) {
	large := bytes.Repeat([]byte("mobile-resource-"), DefaultPAKChunkSize*3/len("mobile-resource-")+17)
	png := bytes.Repeat([]byte{0, 1, 2, 3}, 100)
	path := filepath.Join(t.TempDir(), "assets.pak")
	stats, err := PackPAKEntries(path, []PAKPackSource{
		pakTestSource("data/texture/large.bmp", large),
		pakTestSource("data/texture/icon.png", png),
	}, DefaultPAKOptions())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 2 || stats.Chunks < 4 || stats.Bytes != int64(len(large)+len(png)) {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	pak, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pak.Close()
	if got := pak.Names(); len(got) != 2 || got[0] != "data/texture/icon.png" || got[1] != "data/texture/large.bmp" {
		t.Fatalf("names = %#v", got)
	}
	largeEntry, ok := pak.Entry("DATA\\TEXTURE\\LARGE.BMP")
	if !ok || len(largeEntry.Chunks) < 4 {
		t.Fatalf("large entry = %+v", largeEntry)
	}
	for _, chunk := range largeEntry.Chunks {
		if chunk.Codec != PAKCodecZstd {
			t.Fatalf("large chunk codec = %d, want zstd", chunk.Codec)
		}
	}
	pngEntry, ok := pak.Entry("data/texture/icon.png")
	if !ok || len(pngEntry.Chunks) != 1 || pngEntry.Chunks[0].Codec != PAKCodecRaw {
		t.Fatalf("png entry = %+v", pngEntry)
	}

	reader, size, err := pak.OpenFile("data/texture/large.bmp")
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(large)) {
		t.Fatalf("OpenFile size = %d", size)
	}
	var streamed bytes.Buffer
	buffer := make([]byte, 17)
	for {
		n, readErr := reader.Read(buffer)
		_, _ = streamed.Write(buffer[:n])
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(streamed.Bytes(), large) {
		t.Fatal("streamed PAK resource differs from source")
	}

	got, err := pak.ReadFile("data/texture/large.bmp")
	if err != nil || !bytes.Equal(got, large) {
		t.Fatalf("read len=%d err=%v", len(got), err)
	}
	wantHash := sha256.Sum256(large)
	if largeEntry.SHA256 != wantHash {
		t.Fatalf("resource hash = %x, want %x", largeEntry.SHA256, wantHash)
	}
}

func TestPAKLookupDecodesLegacyEUCKRResourceNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-name.pak")
	name := `data/texture/프론테라/pron-newcastle.bmp`
	if _, err := PackPAKEntries(path, []PAKPackSource{
		pakTestSource(name, []byte("texture")),
	}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}

	legacyName, _, err := transform.String(korean.EUCKR.NewEncoder(), name)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if !archive.Has(legacyName) {
		t.Fatalf("PAK did not match legacy EUC-KR/CP949 name %q", legacyName)
	}
	data, err := archive.ReadFile(legacyName)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "texture" {
		t.Fatalf("legacy-name data = %q, want texture", data)
	}
}

func TestPAKOutputIsDeterministicAndEstimateIsExact(t *testing.T) {
	sources := []PAKPackSource{
		pakTestSource("b.txt", bytes.Repeat([]byte("b"), 1000)),
		pakTestSource("a.txt", bytes.Repeat([]byte("a"), 5000)),
	}
	options := DefaultPAKOptions()
	first := filepath.Join(t.TempDir(), "first.pak")
	second := filepath.Join(t.TempDir(), "second.pak")
	firstStats, err := PackPAKEntries(first, sources, options)
	if err != nil {
		t.Fatal(err)
	}
	secondStats, err := PackPAKEntries(second, []PAKPackSource{sources[0], sources[1]}, options)
	if err != nil {
		t.Fatal(err)
	}
	estimate, err := EstimatePAKEntries(sources, options)
	if err != nil {
		t.Fatal(err)
	}
	firstData, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstData, secondData) {
		t.Fatal("identical PAK inputs produced different bytes")
	}
	if firstStats.ArchiveBytes != int64(len(firstData)) || secondStats.ArchiveBytes != estimate.ArchiveBytes {
		t.Fatalf("archive sizes first=%d second=%d estimate=%d", firstStats.ArchiveBytes, secondStats.ArchiveBytes, estimate.ArchiveBytes)
	}
}

func TestPAKAllowsUnknownSourceSizeAndRecordsStreamedLength(t *testing.T) {
	source := pakTestSource("data/unknown.bin", []byte("streamed size"))
	source.Size = -1
	path := filepath.Join(t.TempDir(), "unknown-size.pak")
	stats, err := PackPAKEntries(path, []PAKPackSource{source}, DefaultPAKOptions())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Bytes != int64(len("streamed size")) {
		t.Fatalf("streamed bytes = %d", stats.Bytes)
	}
	pak, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pak.Close()
	data, err := pak.ReadFile("data/unknown.bin")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "streamed size" {
		t.Fatalf("data = %q", data)
	}
}

func TestPAKRejectsUnsafePathsAndDetectsCorruption(t *testing.T) {
	_, err := EstimatePAKEntries([]PAKPackSource{pakTestSource("../outside", []byte("bad"))}, DefaultPAKOptions())
	if err == nil {
		t.Fatal("unsafe PAK path was accepted")
	}

	path := filepath.Join(t.TempDir(), "corrupt.pak")
	if _, err := PackPAKEntries(path, []PAKPackSource{pakTestSource("data/value.txt", []byte("integrity"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	pak, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := pak.Entry("data/value.txt")
	if !ok {
		t.Fatal("entry missing")
	}
	_ = pak.Close()
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0xff}, int64(entry.Chunks[0].Offset)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	corrupt, err := OpenPAK(path)
	if err != nil {
		t.Fatal(err)
	}
	defer corrupt.Close()
	if _, err := corrupt.ReadFile("data/value.txt"); !errors.Is(err, ErrPAKCorrupt) {
		t.Fatalf("corrupt read error = %v, want ErrPAKCorrupt", err)
	}

	unsupportedPath := filepath.Join(t.TempDir(), "unsupported.pak")
	if _, err := PackPAKEntries(unsupportedPath, []PAKPackSource{pakTestSource("data/value.txt", []byte("version"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	file, err = os.OpenFile(unsupportedPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	version := make([]byte, 4)
	binary.LittleEndian.PutUint32(version, pakVersion+1)
	if _, err := file.WriteAt(version, 8); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	if _, err := OpenPAK(unsupportedPath); !errors.Is(err, ErrPAKUnsupportedVersion) {
		t.Fatalf("unsupported version error = %v", err)
	}

	offsetPath := filepath.Join(t.TempDir(), "offset.pak")
	if _, err := PackPAKEntries(offsetPath, []PAKPackSource{pakTestSource("data/value.txt", []byte("offset"))}, DefaultPAKOptions()); err != nil {
		t.Fatal(err)
	}
	file, err = os.OpenFile(offsetPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	offset := make([]byte, 8)
	binary.LittleEndian.PutUint64(offset, ^uint64(0))
	if _, err := file.WriteAt(offset, 24); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	if _, err := OpenPAK(offsetPath); !errors.Is(err, ErrPAKCorrupt) {
		t.Fatalf("invalid offset error = %v", err)
	}
}

func pakTestSource(name string, data []byte) PAKPackSource {
	data = append([]byte(nil), data...)
	return PAKPackSource{
		Name: name,
		Size: int64(len(data)),
		Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil },
	}
}
