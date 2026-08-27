package res

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

const (
	grfHeaderSize = 46
	grfVersion100 = 0x100
	grfVersion101 = 0x101
	grfVersion102 = 0x102
	grfVersion103 = 0x103
	grfVersion200 = 0x200
	grfVersion300 = 0x300
)

type grfEntryCompression uint8

const (
	grfCompressionZlib grfEntryCompression = iota
	grfCompressionRaw
	grfCompressionLZSS
)

var (
	ErrGRFUnsupportedEncryption = errors.New("grf entry uses unsupported encryption")
	ErrGRFUnsupportedVersion    = errors.New("unsupported grf version")
	ErrGRFNotFound              = errors.New("grf entry not found")
)

type GRF struct {
	path    string
	file    *os.File
	version uint32
	entries map[string]GRFEntry
}

type GRFEntry struct {
	Name        string
	PackedSize  uint32
	AlignedSize uint32
	RealSize    uint32
	Type        byte
	Offset      uint64
	compression grfEntryCompression
	absolute    bool
}

// OpenFile returns a streaming reader for one GRF entry and its uncompressed
// size. Unencrypted entries are decompressed without first materializing the
// complete resource. Encrypted entries retain the existing compatibility path
// and are materialized because the legacy decryptors operate on the entry
// buffer.
func (g *GRF) OpenFile(name string) (io.ReadCloser, int64, error) {
	entry, ok := g.entries[normalizeGRFName(name)]
	if !ok {
		return nil, 0, ErrGRFNotFound
	}
	if entry.absolute || entry.compression != grfCompressionZlib || entry.Type&0x06 != 0 {
		data, err := g.ReadFile(name)
		if err != nil {
			return nil, 0, err
		}
		return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
	}
	section := io.NewSectionReader(g.file, int64(grfEntryFileOffset(entry)), int64(entry.AlignedSize))
	if entry.Type&0x01 == 0 {
		return io.NopCloser(io.LimitReader(section, int64(entry.RealSize))), int64(entry.RealSize), nil
	}
	reader, err := zlib.NewReader(io.LimitReader(section, int64(entry.PackedSize)))
	if err != nil {
		return nil, 0, err
	}
	return &grfExactReader{name: entry.Name, reader: reader, closer: reader, expected: int64(entry.RealSize)}, int64(entry.RealSize), nil
}

func OpenGRF(path string) (*GRF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	grf := &GRF{
		path:    path,
		file:    file,
		entries: make(map[string]GRFEntry),
	}
	if err := grf.load(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return grf, nil
}

func (g *GRF) Close() error {
	if g.file == nil {
		return nil
	}
	return g.file.Close()
}

func (g *GRF) Path() string {
	return g.path
}

func (g *GRF) Count() int {
	return len(g.entries)
}

func (g *GRF) Has(name string) bool {
	_, ok := g.entries[normalizeGRFName(name)]
	return ok
}

// Entry returns the table metadata for one logical resource.
func (g *GRF) Entry(name string) (GRFEntry, bool) {
	entry, ok := g.entries[normalizeGRFName(name)]
	return entry, ok
}

func (g *GRF) Names() []string {
	names := make([]string, 0, len(g.entries))
	for _, entry := range g.entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

func (g *GRF) NamesWithSuffix(suffix string) []string {
	suffix = normalizeGRFName(suffix)
	var names []string
	for key, entry := range g.entries {
		if grfPathSuffixMatch(key, suffix) {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names
}

func grfPathSuffixMatch(name, suffix string) bool {
	if suffix == "" {
		return false
	}
	if name == suffix {
		return true
	}
	return strings.HasSuffix(name, "/"+suffix)
}

func (g *GRF) ReadFile(name string) ([]byte, error) {
	entry, ok := g.entries[normalizeGRFName(name)]
	if !ok {
		return nil, ErrGRFNotFound
	}
	raw := make([]byte, entry.AlignedSize)
	if _, err := g.file.ReadAt(raw, int64(grfEntryFileOffset(entry))); err != nil {
		return nil, err
	}
	if !entry.absolute {
		if err := decryptGRFEntry(raw, entry); err != nil {
			return nil, err
		}
	}

	if entry.compression == grfCompressionLZSS {
		return decompressGRFLZSS(raw[:entry.PackedSize], int64(entry.RealSize), entry.Name)
	}
	if entry.compression == grfCompressionRaw {
		if uint32(len(raw)) > entry.RealSize {
			raw = raw[:entry.RealSize]
		}
		return raw, nil
	}

	if entry.compression != grfCompressionZlib {
		return nil, fmt.Errorf("unsupported grf compression for %s", entry.Name)
	}

	if entry.Type&0x01 == 0 {
		if uint32(len(raw)) > entry.RealSize {
			raw = raw[:entry.RealSize]
		}
		return raw, nil
	}

	reader, err := zlib.NewReader(bytes.NewReader(raw[:entry.PackedSize]))
	if err != nil {
		return nil, err
	}
	out, err := io.ReadAll(io.LimitReader(reader, int64(entry.RealSize)+1))
	closeErr := reader.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(out) != int(entry.RealSize) {
		return nil, fmt.Errorf("corrupt grf entry %s decompressed size %d != %d", entry.Name, len(out), entry.RealSize)
	}
	return out, nil
}

// grfExactReader keeps the streaming GRF API while enforcing the declared
// decompressed length. A zlib stream that produces too few or too many bytes
// must not become a valid resource merely because its table metadata was
// trusted.
type grfExactReader struct {
	name     string
	reader   io.Reader
	closer   io.Closer
	expected int64
	read     int64
	done     bool
	err      error
}

func (r *grfExactReader) Read(data []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.done {
		return 0, io.EOF
	}
	if len(data) == 0 {
		return 0, nil
	}
	if remaining := r.expected - r.read; remaining > 0 {
		if int64(len(data)) > remaining {
			data = data[:remaining]
		}
		n, err := r.reader.Read(data)
		r.read += int64(n)
		if err != nil && err != io.EOF {
			r.err = err
			return n, err
		}
		if err == io.EOF && r.read != r.expected {
			r.err = fmt.Errorf("corrupt grf entry %s decompressed size %d != %d", r.name, r.read, r.expected)
			return n, r.err
		}
		return n, nil
	}
	var extra [1]byte
	n, err := r.reader.Read(extra[:])
	if n != 0 {
		r.err = fmt.Errorf("corrupt grf entry %s decompressed size exceeds %d", r.name, r.expected)
		return 0, r.err
	}
	if err != nil && err != io.EOF {
		r.err = err
		return 0, err
	}
	r.done = true
	if r.closer != nil {
		if closeErr := r.closer.Close(); closeErr != nil {
			r.err = closeErr
			return 0, closeErr
		}
	}
	return 0, io.EOF
}

func (r *grfExactReader) Close() error {
	if r.done || r.closer == nil {
		return nil
	}
	r.done = true
	return r.closer.Close()
}

func decryptGRFEntry(raw []byte, entry GRFEntry) error {
	switch {
	case entry.Type&0x02 != 0:
		if shouldUseHeaderOnlyGRFDecrypt(entry.Name) {
			decryptGRFHeader(raw, entry.AlignedSize)
		} else {
			decryptGRFFull(raw, entry.AlignedSize, entry.PackedSize)
		}
	case entry.Type&0x04 != 0:
		decryptGRFHeader(raw, entry.AlignedSize)
	case entry.Type&^byte(0x07) != 0:
		return fmt.Errorf("%w: %s type=0x%02x", ErrGRFUnsupportedEncryption, entry.Name, entry.Type)
	}
	return nil
}

func shouldUseHeaderOnlyGRFDecrypt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".gnd", ".gat", ".act", ".str":
		return true
	default:
		return false
	}
}

func (g *GRF) load() error {
	header := make([]byte, grfHeaderSize)
	if _, err := io.ReadFull(g.file, header); err != nil {
		return err
	}
	signature := strings.TrimRight(string(header[:15]), "\x00")
	if signature != "Master of Magic" && signature != "Event Horizon" {
		if err := g.loadAlpha(); err == nil {
			return nil
		} else {
			return fmt.Errorf("invalid grf signature %q: %w", signature, err)
		}
	}
	if err := validateGRFEncryptionWatermark(header[15:30]); err != nil {
		return err
	}

	g.version = binary.LittleEndian.Uint32(header[42:46])
	switch g.version {
	case grfVersion100, grfVersion101, grfVersion102, grfVersion103:
		tableOffset := uint64(binary.LittleEndian.Uint32(header[30:34])) + grfHeaderSize
		skip := binary.LittleEndian.Uint32(header[34:38])
		fileCountRaw := binary.LittleEndian.Uint32(header[38:42])
		if fileCountRaw < skip+7 {
			return fmt.Errorf("invalid grf file count")
		}
		return g.loadLegacyTable(tableOffset, fileCountRaw-skip-7)
	case grfVersion200:
		tableOffset := uint64(binary.LittleEndian.Uint32(header[30:34])) + grfHeaderSize
		skip := binary.LittleEndian.Uint32(header[34:38])
		fileCountRaw := binary.LittleEndian.Uint32(header[38:42])
		if fileCountRaw < skip+7 {
			return fmt.Errorf("invalid grf file count")
		}
		return g.loadTable(tableOffset, fileCountRaw-skip-7, false)
	case grfVersion300:
		tableOffset := binary.LittleEndian.Uint64(header[30:38]) + grfHeaderSize + 4
		fileCount := binary.LittleEndian.Uint32(header[38:42])
		return g.loadTable(tableOffset, fileCount, true)
	default:
		return fmt.Errorf("%w: 0x%X", ErrGRFUnsupportedVersion, g.version)
	}
}

// loadAlpha handles the pre-0x100 GRF layout used by early Alpha/Beta
// clients. It has no Master of Magic header; its table offset and entry count
// are stored in a nine-byte trailer instead. Alpha entries use raw bytes or
// the legacy LZSS codec rather than the zlib metadata used by 0x1xx+ GRFs.
func (g *GRF) loadAlpha() error {
	fileSize := g.fileSize()
	if fileSize < grfHeaderSize+9 {
		return fmt.Errorf("%w: alpha trailer is truncated", ErrGRFUnsupportedVersion)
	}
	trailer := make([]byte, 9)
	if _, err := g.file.ReadAt(trailer, fileSize-9); err != nil {
		return err
	}
	tableOffset := uint64(binary.LittleEndian.Uint32(trailer[:4]))
	if tableOffset >= uint64(fileSize-9) {
		return fmt.Errorf("%w: alpha table offset", ErrGRFUnsupportedVersion)
	}
	countWord := binary.LittleEndian.Uint32(trailer[4:8])
	count := (countWord << 16) | (countWord >> 16)
	if tableOffset > uint64(fileSize-9) || uint64(count) > uint64(fileSize-9)-tableOffset {
		return fmt.Errorf("%w: alpha entry count", ErrGRFUnsupportedVersion)
	}
	table := make([]byte, int64(fileSize-9)-int64(tableOffset))
	if _, err := g.file.ReadAt(table, int64(tableOffset)); err != nil {
		return err
	}
	g.version = uint32(trailer[8])
	return g.parseAlphaEntries(table, int(count))
}

func (g *GRF) parseAlphaEntries(table []byte, count int) error {
	position := 0
	for index := 0; index < count; index++ {
		if len(table)-position < 14 {
			return fmt.Errorf("%w: alpha entry %d header", ErrGRFUnsupportedVersion, index)
		}
		nameLength := int(table[position])
		entryType := table[position+1]
		entryOffset := uint64(binary.LittleEndian.Uint32(table[position+2 : position+6]))
		packedSize := binary.LittleEndian.Uint32(table[position+6 : position+10])
		realSize := binary.LittleEndian.Uint32(table[position+10 : position+14])
		position += 14
		if len(table)-position < nameLength+1 {
			return fmt.Errorf("%w: alpha entry %d filename", ErrGRFUnsupportedVersion, index)
		}
		nameBytes := make([]byte, nameLength)
		for nameIndex, value := range table[position : position+nameLength] {
			nameBytes[nameIndex] = (value << 4) | (value >> 4)
		}
		position += nameLength
		if table[position] != 0 {
			return fmt.Errorf("%w: alpha entry %d filename terminator", ErrGRFUnsupportedVersion, index)
		}
		position++
		name := normalizeGRFName(decodeGRFName(nameBytes))
		if name == "" {
			return fmt.Errorf("%w: alpha entry %d empty filename", ErrGRFUnsupportedVersion, index)
		}
		if entryType == 2 {
			continue
		}
		if entryType != 0 && entryType != 1 {
			return fmt.Errorf("%w: alpha entry %s type %d", ErrGRFUnsupportedVersion, name, entryType)
		}
		fileOffset := entryOffset
		if fileOffset > uint64(g.fileSize()) || uint64(packedSize) > uint64(g.fileSize())-fileOffset {
			return fmt.Errorf("%w: alpha entry %s offset", ErrGRFUnsupportedVersion, name)
		}
		compression := grfCompressionRaw
		if entryType == 1 {
			compression = grfCompressionLZSS
		} else if realSize > packedSize {
			return fmt.Errorf("%w: alpha entry %s raw size", ErrGRFUnsupportedVersion, name)
		}
		if _, exists := g.entries[name]; exists {
			return fmt.Errorf("%w: duplicate alpha entry %s", ErrGRFUnsupportedVersion, name)
		}
		g.entries[name] = GRFEntry{
			Name:        name,
			PackedSize:  packedSize,
			AlignedSize: packedSize,
			RealSize:    realSize,
			Type:        entryType,
			Offset:      entryOffset,
			compression: compression,
			absolute:    true,
		}
	}
	return nil
}

func grfEntryFileOffset(entry GRFEntry) uint64 {
	if entry.absolute {
		return entry.Offset
	}
	return entry.Offset + grfHeaderSize
}

func decompressGRFLZSS(compressed []byte, expected int64, name string) ([]byte, error) {
	if expected < 0 || expected > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("corrupt grf entry %s decompressed size", name)
	}
	if expected == 0 {
		return []byte{}, nil
	}
	output := make([]byte, int(expected))
	input := 0
	outputPosition := 0
	for outputPosition < len(output) {
		if input >= len(compressed) {
			return nil, fmt.Errorf("corrupt grf entry %s LZSS stream is truncated", name)
		}
		control := compressed[input]
		input++
		for bit := 0; bit < 8 && outputPosition < len(output); bit++ {
			if control&1 == 0 {
				if input >= len(compressed) {
					return nil, fmt.Errorf("corrupt grf entry %s LZSS literal is truncated", name)
				}
				output[outputPosition] = compressed[input]
				outputPosition++
				input++
			} else {
				if len(compressed)-input < 2 {
					return nil, fmt.Errorf("corrupt grf entry %s LZSS phrase is truncated", name)
				}
				codeword := binary.LittleEndian.Uint16(compressed[input : input+2])
				input += 2
				phraseLength := int((codeword >> 12) + 2)
				phraseIndex := int(codeword & 0x0fff)
				if phraseIndex <= 0 || phraseIndex > outputPosition || phraseLength > len(output)-outputPosition {
					return nil, fmt.Errorf("corrupt grf entry %s LZSS phrase", name)
				}
				for phrase := 0; phrase < phraseLength; phrase++ {
					output[outputPosition] = output[outputPosition-phraseIndex]
					outputPosition++
				}
			}
			control >>= 1
		}
	}
	return output, nil
}

func validateGRFEncryptionWatermark(watermark []byte) error {
	if len(watermark) != 15 {
		return fmt.Errorf("%w: invalid encryption watermark length", ErrGRFUnsupportedEncryption)
	}
	allZero := true
	sequential := true
	for i, value := range watermark {
		if value != 0 {
			allZero = false
		}
		if value != byte(i) {
			sequential = false
		}
	}
	if !allZero && !sequential {
		return fmt.Errorf("%w: unsupported encryption watermark", ErrGRFUnsupportedEncryption)
	}
	return nil
}

// loadLegacyTable reads the uncompressed 0x1xx GRF/GPF index. These archives
// use an older name encoding and store obfuscated size fields in the table;
// the data section still uses the same GRF entry encryption flags.
func (g *GRF) loadLegacyTable(offset uint64, count uint32) error {
	if offset > uint64(g.fileSize()) {
		return fmt.Errorf("corrupt grf table offset")
	}
	table := make([]byte, uint64(g.fileSize())-offset)
	if _, err := g.file.ReadAt(table, int64(offset)); err != nil {
		return err
	}
	return g.parseLegacyEntries(table, int(count), g.version)
}

func (g *GRF) parseLegacyEntries(table []byte, count int, version uint32) error {
	pos := 0
	for i := 0; i < count; i++ {
		if len(table)-pos < 4 {
			return fmt.Errorf("truncated grf legacy filename length at entry %d", i)
		}
		nameLength := binary.LittleEndian.Uint32(table[pos : pos+4])
		pos += 4
		if nameLength == 0 || uint64(nameLength) > uint64(len(table)-pos) {
			return fmt.Errorf("corrupt grf legacy filename length at entry %d", i)
		}

		var nameBytes []byte
		if version < grfVersion101 {
			nameBytes = make([]byte, nameLength)
			for j, value := range table[pos : pos+int(nameLength)] {
				nameBytes[j] = (value << 4) | (value >> 4)
			}
		} else {
			if nameLength < 6 {
				return fmt.Errorf("corrupt grf legacy filename length at entry %d", i)
			}
			encodedLength := nameLength - 6
			if uint64(encodedLength) > uint64(len(table)-pos-2) {
				return fmt.Errorf("truncated grf legacy filename at entry %d", i)
			}
			nameBytes = make([]byte, encodedLength)
			for j, value := range table[pos+2 : pos+2+int(encodedLength)] {
				nameBytes[j] = (value << 4) | (value >> 4)
			}
			// GRF 0x101-0x103 uses the intentionally broken, zero-key
			// one-round DES variant for its index names.
			decryptGRFHeader(nameBytes, uint32(len(nameBytes)))
		}
		nameEnd := 0
		for nameEnd < len(nameBytes) && nameBytes[nameEnd] != 0 {
			nameEnd++
		}
		if nameEnd == 0 {
			return fmt.Errorf("empty grf legacy filename at entry %d", i)
		}
		name := normalizeGRFName(decodeGRFName(nameBytes[:nameEnd]))
		if name == "" {
			return fmt.Errorf("empty grf legacy filename at entry %d", i)
		}
		pos += int(nameLength)
		if len(table)-pos < 17 {
			return fmt.Errorf("truncated grf legacy metadata at entry %d", i)
		}

		packedEncoded := binary.LittleEndian.Uint32(table[pos : pos+4])
		alignedEncoded := binary.LittleEndian.Uint32(table[pos+4 : pos+8])
		realSize := binary.LittleEndian.Uint32(table[pos+8 : pos+12])
		if packedEncoded < realSize+0x02CB || alignedEncoded < 0x92CB {
			return fmt.Errorf("corrupt grf legacy sizes for %s", name)
		}
		packedSize := packedEncoded - realSize - 0x02CB
		alignedSize := alignedEncoded - 0x92CB
		rawType := table[pos+12]
		if rawType&^byte(0x01) != 0 {
			return fmt.Errorf("%w: %s type=0x%02x", ErrGRFUnsupportedEncryption, name, rawType)
		}
		entryType := rawType
		if isGRFSpecialEntry(name) {
			entryType |= 0x04
		} else {
			entryType |= 0x02
		}
		entryOffset := uint64(binary.LittleEndian.Uint32(table[pos+13 : pos+17]))
		fileOffset := entryOffset + grfHeaderSize
		if fileOffset < entryOffset || fileOffset > uint64(g.fileSize()) || uint64(alignedSize) > uint64(g.fileSize())-fileOffset {
			return fmt.Errorf("corrupt grf legacy entry %s offset", name)
		}
		if alignedSize < packedSize || (entryType&0x01) == 0 && realSize > alignedSize {
			return fmt.Errorf("corrupt grf legacy entry %s sizes", name)
		}
		if _, exists := g.entries[name]; exists {
			return fmt.Errorf("duplicate grf entry %s", name)
		}
		g.entries[name] = GRFEntry{
			Name:        name,
			PackedSize:  packedSize,
			AlignedSize: alignedSize,
			RealSize:    realSize,
			Type:        entryType,
			Offset:      entryOffset,
		}
		pos += 17
	}
	return nil
}

func isGRFSpecialEntry(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".gnd", ".gat", ".act", ".str":
		return true
	default:
		return false
	}
}

func (g *GRF) loadTable(offset uint64, count uint32, wideOffset bool) error {
	if offset > uint64(g.fileSize()) || uint64(8) > uint64(g.fileSize())-offset {
		return fmt.Errorf("corrupt grf table offset")
	}
	tableHeader := make([]byte, 8)
	if _, err := g.file.ReadAt(tableHeader, int64(offset)); err != nil {
		return err
	}
	packedSize := binary.LittleEndian.Uint32(tableHeader[:4])
	realSize := binary.LittleEndian.Uint32(tableHeader[4:8])
	if uint64(packedSize) > uint64(g.fileSize())-offset-8 {
		return fmt.Errorf("corrupt grf table size")
	}

	packed := make([]byte, packedSize)
	if _, err := g.file.ReadAt(packed, int64(offset+8)); err != nil {
		return err
	}

	reader, err := zlib.NewReader(bytes.NewReader(packed))
	if err != nil {
		return err
	}
	defer reader.Close()

	table, err := io.ReadAll(io.LimitReader(reader, int64(realSize)+1))
	closeErr := reader.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if len(table) != int(realSize) {
		return fmt.Errorf("corrupt grf table size %d != %d", len(table), realSize)
	}
	return g.parseEntries(table, int(count), wideOffset)
}

func (g *GRF) parseEntries(table []byte, count int, wideOffset bool) error {
	pos := 0
	for i := 0; i < count; i++ {
		start := pos
		for pos < len(table) && table[pos] != 0 {
			pos++
		}
		if pos >= len(table) {
			return fmt.Errorf("unterminated grf filename at entry %d", i)
		}
		name := normalizeGRFName(decodeGRFName(table[start:pos]))
		pos++

		metaSize := 17
		if wideOffset {
			metaSize = 21
		}
		if len(table)-pos < metaSize {
			return fmt.Errorf("truncated grf metadata at entry %d", i)
		}

		entry := GRFEntry{
			Name:        name,
			PackedSize:  binary.LittleEndian.Uint32(table[pos : pos+4]),
			AlignedSize: binary.LittleEndian.Uint32(table[pos+4 : pos+8]),
			RealSize:    binary.LittleEndian.Uint32(table[pos+8 : pos+12]),
			Type:        table[pos+12],
		}
		if wideOffset {
			entry.Offset = binary.LittleEndian.Uint64(table[pos+13 : pos+21])
		} else {
			entry.Offset = uint64(binary.LittleEndian.Uint32(table[pos+13 : pos+17]))
		}
		if entry.AlignedSize < entry.PackedSize {
			return fmt.Errorf("corrupt grf entry %s has aligned size smaller than packed size", name)
		}
		if entry.Type&0x01 == 0 && entry.RealSize > entry.AlignedSize {
			return fmt.Errorf("corrupt grf entry %s has real size larger than stored size", name)
		}
		if entry.Type&^byte(0x07) != 0 {
			return fmt.Errorf("%w: %s type=0x%02x", ErrGRFUnsupportedEncryption, name, entry.Type)
		}
		fileOffset := entry.Offset + grfHeaderSize
		if fileOffset < entry.Offset || fileOffset > uint64(g.fileSize()) || uint64(entry.AlignedSize) > uint64(g.fileSize())-fileOffset {
			return fmt.Errorf("corrupt grf entry %s offset", name)
		}
		pos += metaSize

		g.entries[name] = entry
	}
	return nil
}

func (g *GRF) fileSize() int64 {
	if g == nil || g.file == nil {
		return 0
	}
	info, err := g.file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

func normalizeGRFName(name string) string {
	if !utf8.ValidString(name) {
		name = decodeGRFName([]byte(name))
	}
	name = strings.TrimLeft(name, `\/`)
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.EqualFold(name, "root") {
		return ""
	}
	if len(name) >= len("root/") && strings.EqualFold(name[:len("root/")], "root/") {
		name = name[len("root/"):]
	}
	return strings.ToLower(name)
}

// NormalizeResourcePath converts a client resource path to the same canonical
// spelling used by GRF lookup, including EUC-KR table-name decoding.
func NormalizeResourcePath(name string) string {
	return normalizeGRFName(name)
}

func decodeGRFName(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, _, err := transform.Bytes(korean.EUCKR.NewDecoder(), data)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}
