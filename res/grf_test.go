package res

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGRFReadPlainCompressedEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.grf")

	if err := writeTestGRF(path, "data\\hello.txt", []byte("hello world")); err != nil {
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

	if grf.Count() != 1 {
		t.Fatalf("count = %d", grf.Count())
	}

	data, err := grf.ReadFile("data/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("data = %q", string(data))
	}
}

func TestNormalizeGRFNameDecodesEUCToUTF8(t *testing.T) {
	input := "DATA/" + string([]byte{0xb8, 0xf3, 0xbd, 0xba, 0xc5, 0xcd}) + "/PORING.SPR"
	got := normalizeGRFName(input)
	want := "data/몬스터/poring.spr"
	if got != want {
		t.Fatalf("normalizeGRFName = %q, want %q", got, want)
	}
}

func TestGRFNamesWithSuffixRequiresPathBoundary(t *testing.T) {
	grf := &GRF{
		entries: map[string]GRFEntry{
			normalizeGRFName(`data\sprite\monster\orc_zombie.spr`): {Name: `data/sprite/monster/orc_zombie.spr`},
			normalizeGRFName(`data\sprite\monster\zombie.spr`):     {Name: `data/sprite/monster/zombie.spr`},
		},
	}

	matches := grf.NamesWithSuffix("zombie.spr")
	if len(matches) != 1 || matches[0] != `data/sprite/monster/zombie.spr` {
		t.Fatalf("matches = %#v, want only zombie.spr", matches)
	}
}

func TestGRFNamesWithSuffixFindsFileUnderDifferentRoot(t *testing.T) {
	grf := &GRF{
		entries: map[string]GRFEntry{
			normalizeGRFName(`data\prontera.gat`): {Name: `data/prontera.gat`},
		},
	}

	matches := grf.NamesWithSuffix("prontera.gat")
	if len(matches) != 1 || matches[0] != `data/prontera.gat` {
		t.Fatalf("matches = %#v, want data prontera", matches)
	}
}

func TestGRFReadKoreanEntryWithUTF8Path(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.grf")
	rawName := "data\\sprite\\" + string([]byte{0xb8, 0xf3, 0xbd, 0xba, 0xc5, 0xcd}) + "\\poring.spr"

	if err := writeTestGRF(path, rawName, []byte("sprite")); err != nil {
		t.Fatal(err)
	}

	grf, err := OpenGRF(path)
	if err != nil {
		t.Fatal(err)
	}
	defer grf.Close()

	if !grf.Has("data/sprite/몬스터/poring.spr") {
		t.Fatal("UTF-8 Korean path was not indexed")
	}
	data, err := grf.ReadFile("data/sprite/몬스터/poring.spr")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sprite" {
		t.Fatalf("data = %q", data)
	}
}

func TestPackGRFRoundTripWithKoreanPath(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(root, "data", "sprite", "몬스터"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "sprite", "몬스터", "poring.spr"), []byte("poring"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "packed.grf")
	stats, err := PackGRF(path, root)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 1 || stats.Bytes != int64(len("poring")) {
		t.Fatalf("stats = %+v", stats)
	}

	grf, err := OpenGRF(path)
	if err != nil {
		t.Fatal(err)
	}
	defer grf.Close()

	if !grf.Has("data/sprite/몬스터/poring.spr") {
		t.Fatal("packed UTF-8 Korean path not found")
	}
	data, err := grf.ReadFile("data/sprite/몬스터/poring.spr")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "poring" {
		t.Fatalf("data = %q", data)
	}
}

func TestGRFReadAlphaRawEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.grf")
	if err := writeTestAlphaGRF(path, `data\alpha.txt`, []byte("alpha data")); err != nil {
		t.Fatal(err)
	}

	grf, err := OpenGRF(path)
	if err != nil {
		t.Fatal(err)
	}
	defer grf.Close()
	if grf.Count() != 1 || !grf.Has("data/alpha.txt") {
		t.Fatalf("alpha entries = %d, names = %#v", grf.Count(), grf.Names())
	}
	data, err := grf.ReadFile("data/alpha.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alpha data" {
		t.Fatalf("alpha data = %q", data)
	}
}

func TestGRFRealArchiveWhenConfigured(t *testing.T) {
	grf := realDataArchive(t)
	name := "prontera.gat"

	if grf.Count() == 0 {
		t.Fatal("archive has no entries")
	}
	if !grf.Has(name) {
		matches := grf.NamesWithSuffix(name)
		if len(matches) == 0 {
			t.Skipf("%s not present in %s", name, grf.Path())
		}
		name = matches[0]
	}
	data, err := grf.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if len(data) == 0 {
		t.Fatalf("read %s returned empty data", name)
	}
}

func writeTestGRF(path, name string, content []byte) error {
	compressedFile := zlibBytes(content)

	var table bytes.Buffer
	table.WriteString(name)
	table.WriteByte(0)
	writeU32(&table, uint32(len(compressedFile)))
	writeU32(&table, uint32(len(compressedFile)))
	writeU32(&table, uint32(len(content)))
	table.WriteByte(0x01)
	writeU32(&table, 0)

	compressedTable := zlibBytes(table.Bytes())

	var out bytes.Buffer
	header := make([]byte, grfHeaderSize)
	copy(header[:15], []byte("Master of Magic"))
	binary.LittleEndian.PutUint32(header[30:34], uint32(len(compressedFile)))
	binary.LittleEndian.PutUint32(header[34:38], 0)
	binary.LittleEndian.PutUint32(header[38:42], 8)
	binary.LittleEndian.PutUint32(header[42:46], grfVersion200)
	out.Write(header)
	out.Write(compressedFile)
	writeU32(&out, uint32(len(compressedTable)))
	writeU32(&out, uint32(len(table.Bytes())))
	out.Write(compressedTable)

	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeTestAlphaGRF(path, name string, content []byte) error {
	const dataOffset = 46
	var table bytes.Buffer
	nameBytes := []byte(name)
	if len(nameBytes) > 255 {
		return errors.New("alpha test name is too long")
	}
	table.WriteByte(byte(len(nameBytes)))
	table.WriteByte(0)
	writeU32(&table, dataOffset)
	writeU32(&table, uint32(len(content)))
	writeU32(&table, uint32(len(content)))
	for _, value := range nameBytes {
		table.WriteByte((value << 4) | (value >> 4))
	}
	table.WriteByte(0)

	var out bytes.Buffer
	out.Write(make([]byte, dataOffset))
	out.Write(content)
	tableOffset := out.Len()
	out.Write(table.Bytes())
	writeU32(&out, uint32(tableOffset))
	writeU32(&out, 0x00010000)
	out.WriteByte(0x18)
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func zlibBytes(data []byte) []byte {
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return out.Bytes()
}

func writeU32(buf *bytes.Buffer, value uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], value)
	buf.Write(tmp[:])
}
