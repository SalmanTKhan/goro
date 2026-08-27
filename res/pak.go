package res

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	pakHeaderSize       = 64
	pakVersion          = 1
	pakMagic            = "GOROPAK\x00"
	DefaultPAKChunkSize = 256 * 1024
	DefaultPAKZstdLevel = 9
)

var (
	ErrPAKUnsupportedVersion = errors.New("unsupported pak version")
	ErrPAKNotFound           = errors.New("pak entry not found")
	ErrPAKUnsupportedCodec   = errors.New("unsupported pak codec")
	ErrPAKCorrupt            = errors.New("corrupt pak")
)

type PAKCodec uint32

const (
	PAKCodecRaw  PAKCodec = 0
	PAKCodecZstd PAKCodec = 1
)

type PAKOptions struct {
	ChunkSize   int
	ZstdLevel   int
	RawSuffixes []string
}

func DefaultPAKOptions() PAKOptions {
	return PAKOptions{
		ChunkSize: DefaultPAKChunkSize,
		ZstdLevel: DefaultPAKZstdLevel,
	}
}

type PAKPackSource struct {
	Name string
	// Size is the declared uncompressed size. Set it to -1 when the source
	// length is unknown; the writer will determine it while streaming.
	Size int64
	Open func() (io.ReadCloser, error)
}

type PAKPackStats struct {
	Files           int
	Chunks          int
	Bytes           int64
	CompressedBytes int64
	ArchiveBytes    int64
}

type PAKChunk struct {
	Offset           uint64
	CompressedSize   uint32
	UncompressedSize uint32
	Codec            PAKCodec
	DictionaryID     uint32
	Checksum         uint32
}

type PAKEntry struct {
	Name             string
	UncompressedSize uint64
	SHA256           [sha256.Size]byte
	Chunks           []PAKChunk
}

type PAK struct {
	path      string
	file      *os.File
	fileSize  int64
	chunkSize uint32
	entries   map[string]PAKEntry
}

func PackPAK(outputPath string, root string, options PAKOptions) (PAKPackStats, error) {
	files, err := collectGRFFiles(root)
	if err != nil {
		return PAKPackStats{}, err
	}
	sources := make([]PAKPackSource, 0, len(files))
	for _, name := range files {
		sourcePath := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Stat(sourcePath)
		if err != nil {
			return PAKPackStats{}, err
		}
		resourceName := name
		sources = append(sources, PAKPackSource{
			Name: resourceName,
			Size: info.Size(),
			Open: func() (io.ReadCloser, error) { return os.Open(sourcePath) },
		})
	}
	return PackPAKEntries(outputPath, sources, options)
}

func PackPAKEntries(outputPath string, sources []PAKPackSource, options PAKOptions) (PAKPackStats, error) {
	options, err := normalizePAKOptions(options)
	if err != nil {
		return PAKPackStats{}, err
	}
	out, err := os.Create(outputPath)
	if err != nil {
		return PAKPackStats{}, err
	}
	closeOutput := true
	defer func() {
		if closeOutput {
			_ = out.Close()
		}
	}()

	stats, err := writePAK(out, sources, options)
	if err != nil {
		return PAKPackStats{}, err
	}
	if err := out.Close(); err != nil {
		return PAKPackStats{}, err
	}
	closeOutput = false
	return stats, nil
}

func EstimatePAKEntries(sources []PAKPackSource, options PAKOptions) (PAKPackStats, error) {
	options, err := normalizePAKOptions(options)
	if err != nil {
		return PAKPackStats{}, err
	}
	return writePAK(&pakCountingWriter{}, sources, options)
}

func OpenPAK(filePath string) (*PAK, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	pak := &PAK{path: filePath, file: file, fileSize: info.Size(), entries: make(map[string]PAKEntry)}
	if err := pak.load(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return pak, nil
}

func (p *PAK) Path() string { return p.path }

func (p *PAK) Close() error {
	if p == nil || p.file == nil {
		return nil
	}
	return p.file.Close()
}

func (p *PAK) Count() int { return len(p.entries) }

func (p *PAK) Has(name string) bool {
	_, ok := p.entries[pakLookupName(name)]
	return ok
}

func (p *PAK) Entry(name string) (PAKEntry, bool) {
	entry, ok := p.entries[pakLookupName(name)]
	if !ok {
		return PAKEntry{}, false
	}
	entry.Chunks = append([]PAKChunk(nil), entry.Chunks...)
	return entry, true
}

func (p *PAK) Names() []string {
	names := make([]string, 0, len(p.entries))
	for _, entry := range p.entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

func (p *PAK) NamesWithSuffix(suffix string) []string {
	suffix = pakLookupName(suffix)
	var names []string
	for key, entry := range p.entries {
		if grfPathSuffixMatch(key, suffix) {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names
}

func (p *PAK) OpenFile(name string) (io.ReadCloser, int64, error) {
	entry, ok := p.entries[pakLookupName(name)]
	if !ok {
		return nil, 0, ErrPAKNotFound
	}
	return &pakEntryReader{pak: p, entry: entry, digest: sha256.New()}, int64(entry.UncompressedSize), nil
}

func (p *PAK) ReadFile(name string) ([]byte, error) {
	reader, _, err := p.OpenFile(name)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (p *PAK) load() error {
	if p.fileSize < pakHeaderSize {
		return fmt.Errorf("%w: file shorter than header", ErrPAKCorrupt)
	}
	header := make([]byte, pakHeaderSize)
	if _, err := p.file.ReadAt(header, 0); err != nil {
		return err
	}
	if string(header[:8]) != pakMagic {
		return fmt.Errorf("%w: invalid magic", ErrPAKCorrupt)
	}
	if binary.LittleEndian.Uint32(header[8:12]) != pakVersion {
		return fmt.Errorf("%w: %d", ErrPAKUnsupportedVersion, binary.LittleEndian.Uint32(header[8:12]))
	}
	if binary.LittleEndian.Uint32(header[12:16]) != pakHeaderSize {
		return fmt.Errorf("%w: invalid header size", ErrPAKCorrupt)
	}
	p.chunkSize = binary.LittleEndian.Uint32(header[16:20])
	if p.chunkSize == 0 {
		return fmt.Errorf("%w: zero chunk size", ErrPAKCorrupt)
	}
	entryCount := binary.LittleEndian.Uint32(header[20:24])
	indexOffset := binary.LittleEndian.Uint64(header[24:32])
	indexSize := binary.LittleEndian.Uint64(header[32:40])
	if indexOffset < pakHeaderSize || indexOffset > uint64(p.fileSize) || indexSize > uint64(p.fileSize)-indexOffset {
		return fmt.Errorf("%w: index outside file", ErrPAKCorrupt)
	}
	if indexSize > uint64(maxInt()) {
		return fmt.Errorf("%w: index too large", ErrPAKCorrupt)
	}
	index := make([]byte, int(indexSize))
	if _, err := p.file.ReadAt(index, int64(indexOffset)); err != nil {
		return err
	}
	if uint64(entryCount) > uint64(len(index))/48+1 {
		return fmt.Errorf("%w: entry count exceeds index", ErrPAKCorrupt)
	}
	reader := pakIndexReader{data: index}
	for i := uint32(0); i < entryCount; i++ {
		name, err := reader.string()
		if err != nil {
			return fmt.Errorf("%w: entry %d name: %v", ErrPAKCorrupt, i, err)
		}
		canonical, err := normalizePAKName(name)
		if err != nil {
			return fmt.Errorf("%w: entry %d name: %v", ErrPAKCorrupt, i, err)
		}
		entry := PAKEntry{Name: canonical}
		entry.UncompressedSize, err = reader.u64()
		if err != nil {
			return fmt.Errorf("%w: entry %d size: %v", ErrPAKCorrupt, i, err)
		}
		if entry.UncompressedSize > uint64(maxInt()) || entry.UncompressedSize > uint64(^uint64(0)>>1) {
			return fmt.Errorf("%w: entry %d is too large", ErrPAKCorrupt, i)
		}
		hashBytes, err := reader.bytes(sha256.Size)
		if err != nil {
			return fmt.Errorf("%w: entry %d hash: %v", ErrPAKCorrupt, i, err)
		}
		copy(entry.SHA256[:], hashBytes)
		chunkCount, err := reader.u32()
		if err != nil {
			return fmt.Errorf("%w: entry %d chunk count: %v", ErrPAKCorrupt, i, err)
		}
		if uint64(chunkCount)*28 > uint64(reader.remaining()) {
			return fmt.Errorf("%w: entry %d chunk count exceeds index", ErrPAKCorrupt, i)
		}
		entry.Chunks = make([]PAKChunk, chunkCount)
		var total uint64
		for chunkIndex := range entry.Chunks {
			chunk := &entry.Chunks[chunkIndex]
			chunk.Offset, err = reader.u64()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d offset: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			chunk.CompressedSize, err = reader.u32()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d compressed size: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			chunk.UncompressedSize, err = reader.u32()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d uncompressed size: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			codec, err := reader.u32()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d codec: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			chunk.Codec = PAKCodec(codec)
			chunk.DictionaryID, err = reader.u32()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d dictionary: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			chunk.Checksum, err = reader.u32()
			if err != nil {
				return fmt.Errorf("%w: entry %d chunk %d checksum: %v", ErrPAKCorrupt, i, chunkIndex, err)
			}
			if chunk.DictionaryID != 0 {
				return fmt.Errorf("%w: entry %d chunk %d uses dictionary %d", ErrPAKCorrupt, i, chunkIndex, chunk.DictionaryID)
			}
			if chunk.Codec != PAKCodecRaw && chunk.Codec != PAKCodecZstd {
				return fmt.Errorf("%w: entry %d chunk %d codec %d", ErrPAKUnsupportedCodec, i, chunkIndex, chunk.Codec)
			}
			if uint64(chunk.UncompressedSize) > uint64(p.chunkSize) {
				return fmt.Errorf("%w: entry %d chunk %d exceeds chunk size", ErrPAKCorrupt, i, chunkIndex)
			}
			if chunk.Offset < pakHeaderSize || chunk.Offset > indexOffset || uint64(chunk.CompressedSize) > indexOffset-chunk.Offset {
				return fmt.Errorf("%w: entry %d chunk %d outside file", ErrPAKCorrupt, i, chunkIndex)
			}
			total += uint64(chunk.UncompressedSize)
		}
		if total != entry.UncompressedSize {
			return fmt.Errorf("%w: entry %d size %d != chunks %d", ErrPAKCorrupt, i, entry.UncompressedSize, total)
		}
		key := pakLookupName(canonical)
		if _, exists := p.entries[key]; exists {
			return fmt.Errorf("%w: duplicate entry %s", ErrPAKCorrupt, canonical)
		}
		p.entries[key] = entry
	}
	if reader.remaining() != 0 {
		return fmt.Errorf("%w: trailing index bytes", ErrPAKCorrupt)
	}
	return nil
}

type pakEntryReader struct {
	pak       *PAK
	entry     PAKEntry
	chunk     int
	current   *bytes.Reader
	closed    bool
	readError error
	digest    hash.Hash
	verified  bool
}

func (r *pakEntryReader) Read(data []byte) (int, error) {
	if r.closed {
		return 0, os.ErrClosed
	}
	if r.readError != nil {
		return 0, r.readError
	}
	for {
		if r.current != nil {
			if r.current.Len() > 0 {
				n, err := r.current.Read(data)
				if n > 0 {
					_, _ = r.digest.Write(data[:n])
				}
				return n, err
			}
			r.current = nil
		}
		if r.chunk >= len(r.entry.Chunks) {
			if !r.verified {
				r.verified = true
				if !bytes.Equal(r.digest.Sum(nil), r.entry.SHA256[:]) {
					r.readError = fmt.Errorf("%w: resource hash mismatch", ErrPAKCorrupt)
					return 0, r.readError
				}
			}
			return 0, io.EOF
		}
		chunk, err := r.pak.readChunk(r.entry.Chunks[r.chunk])
		if err != nil {
			r.readError = err
			return 0, err
		}
		r.chunk++
		r.current = bytes.NewReader(chunk)
	}
}

func (r *pakEntryReader) Close() error {
	r.closed = true
	r.current = nil
	return nil
}

func (p *PAK) readChunk(chunk PAKChunk) ([]byte, error) {
	if chunk.CompressedSize > uint32(maxInt()) || chunk.UncompressedSize > uint32(maxInt()) {
		return nil, fmt.Errorf("%w: chunk too large", ErrPAKCorrupt)
	}
	if chunk.Offset > uint64(p.fileSize) || uint64(chunk.CompressedSize) > uint64(p.fileSize)-chunk.Offset {
		return nil, fmt.Errorf("%w: chunk outside file", ErrPAKCorrupt)
	}
	compressed := make([]byte, int(chunk.CompressedSize))
	if _, err := p.file.ReadAt(compressed, int64(chunk.Offset)); err != nil {
		return nil, err
	}
	var decoded []byte
	switch chunk.Codec {
	case PAKCodecRaw:
		if chunk.CompressedSize != chunk.UncompressedSize {
			return nil, fmt.Errorf("%w: raw chunk has different sizes", ErrPAKCorrupt)
		}
		decoded = compressed
	case PAKCodecZstd:
		decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		decoded, err = decoder.DecodeAll(compressed, nil)
		decoder.Close()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: %d", ErrPAKUnsupportedCodec, chunk.Codec)
	}
	if uint32(len(decoded)) != chunk.UncompressedSize {
		return nil, fmt.Errorf("%w: decoded size %d != %d", ErrPAKCorrupt, len(decoded), chunk.UncompressedSize)
	}
	if crc32.Checksum(decoded, pakCRC32CTable) != chunk.Checksum {
		return nil, fmt.Errorf("%w: chunk checksum mismatch", ErrPAKCorrupt)
	}
	return decoded, nil
}

var pakCRC32CTable = crc32.MakeTable(crc32.Castagnoli)

type pakWriteTarget struct {
	w      io.Writer
	offset int64
}

func (w *pakWriteTarget) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.offset += int64(n)
	return n, err
}

type pakCountingWriter struct{ offset int64 }

func (w *pakCountingWriter) Write(data []byte) (int, error) {
	w.offset += int64(len(data))
	return len(data), nil
}

func writePAK(output io.Writer, sources []PAKPackSource, options PAKOptions) (PAKPackStats, error) {
	sources = append([]PAKPackSource(nil), sources...)
	for index := range sources {
		canonical, err := normalizePAKName(sources[index].Name)
		if err != nil {
			return PAKPackStats{}, fmt.Errorf("source %q: %w", sources[index].Name, err)
		}
		sources[index].Name = canonical
	}
	sort.SliceStable(sources, func(i, j int) bool {
		left := pakLookupName(sources[i].Name)
		right := pakLookupName(sources[j].Name)
		if left == right {
			return sources[i].Name < sources[j].Name
		}
		return left < right
	})
	for index := 1; index < len(sources); index++ {
		if pakLookupName(sources[index-1].Name) == pakLookupName(sources[index].Name) {
			return PAKPackStats{}, fmt.Errorf("duplicate PAK source %q", sources[index].Name)
		}
	}

	target := &pakWriteTarget{w: output}
	if _, err := target.Write(make([]byte, pakHeaderSize)); err != nil {
		return PAKPackStats{}, err
	}
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(options.ZstdLevel)), zstd.WithEncoderConcurrency(1))
	if err != nil {
		return PAKPackStats{}, err
	}
	defer encoder.Close()

	entries := make([]PAKEntry, 0, len(sources))
	var stats PAKPackStats
	for _, source := range sources {
		if source.Name == "" || source.Open == nil || source.Size < -1 {
			return PAKPackStats{}, fmt.Errorf("invalid PAK source %q", source.Name)
		}
		reader, err := source.Open()
		if err != nil {
			return PAKPackStats{}, fmt.Errorf("open %s: %w", source.Name, err)
		}
		entry, copied, chunks, compressedBytes, err := writePAKEntry(target, reader, source, options, encoder)
		closeErr := reader.Close()
		if err != nil {
			return PAKPackStats{}, fmt.Errorf("write %s: %w", source.Name, err)
		}
		if closeErr != nil {
			return PAKPackStats{}, fmt.Errorf("close %s: %w", source.Name, closeErr)
		}
		if source.Size >= 0 && copied != source.Size {
			return PAKPackStats{}, fmt.Errorf("%s: streamed size %d does not match declared size %d", source.Name, copied, source.Size)
		}
		entries = append(entries, entry)
		stats.Files++
		stats.Chunks += chunks
		stats.Bytes += copied
		stats.CompressedBytes += compressedBytes
	}

	indexOffset := target.offset
	index, err := encodePAKIndex(entries)
	if err != nil {
		return PAKPackStats{}, err
	}
	if _, err := target.Write(index); err != nil {
		return PAKPackStats{}, err
	}
	stats.ArchiveBytes = target.offset

	if file, ok := output.(interface {
		io.Writer
		Seek(int64, int) (int64, error)
	}); ok {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return PAKPackStats{}, err
		}
		if err := writePAKHeader(file, uint32(options.ChunkSize), uint32(len(entries)), uint64(indexOffset), uint64(len(index))); err != nil {
			return PAKPackStats{}, err
		}
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			return PAKPackStats{}, err
		}
	}
	return stats, nil
}

func writePAKEntry(target *pakWriteTarget, reader io.Reader, source PAKPackSource, options PAKOptions, encoder *zstd.Encoder) (PAKEntry, int64, int, int64, error) {
	entry := PAKEntry{Name: source.Name}
	digest := sha256.New()
	buffer := make([]byte, options.ChunkSize)
	var copied int64
	var compressedBytes int64
	for {
		n, readErr := io.ReadFull(reader, buffer)
		if n > 0 {
			chunkData := buffer[:n]
			if _, err := digest.Write(chunkData); err != nil {
				return PAKEntry{}, copied, len(entry.Chunks), compressedBytes, err
			}
			codec := PAKCodecZstd
			var compressed []byte
			if isPAKRawSource(source.Name, options) {
				codec = PAKCodecRaw
				compressed = chunkData
			} else {
				compressed = encoder.EncodeAll(chunkData, nil)
				if len(compressed) >= len(chunkData) {
					codec = PAKCodecRaw
					compressed = chunkData
				}
			}
			if uint64(target.offset) > uint64(^uint64(0))-uint64(len(compressed)) {
				return PAKEntry{}, copied, len(entry.Chunks), compressedBytes, fmt.Errorf("PAK offset overflow")
			}
			chunk := PAKChunk{Offset: uint64(target.offset), CompressedSize: uint32(len(compressed)), UncompressedSize: uint32(n), Codec: codec, Checksum: crc32.Checksum(chunkData, pakCRC32CTable)}
			if _, err := target.Write(compressed); err != nil {
				return PAKEntry{}, copied, len(entry.Chunks), compressedBytes, err
			}
			entry.Chunks = append(entry.Chunks, chunk)
			entry.UncompressedSize += uint64(n)
			copied += int64(n)
			compressedBytes += int64(len(compressed))
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		return PAKEntry{}, copied, len(entry.Chunks), compressedBytes, readErr
	}
	copy(entry.SHA256[:], digest.Sum(nil))
	return entry, copied, len(entry.Chunks), compressedBytes, nil
}

func encodePAKIndex(entries []PAKEntry) ([]byte, error) {
	var index bytes.Buffer
	for _, entry := range entries {
		if uint64(len(entry.Name)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("PAK name too long: %s", entry.Name)
		}
		if err := binary.Write(&index, binary.LittleEndian, uint32(len(entry.Name))); err != nil {
			return nil, err
		}
		if _, err := index.WriteString(entry.Name); err != nil {
			return nil, err
		}
		if err := binary.Write(&index, binary.LittleEndian, entry.UncompressedSize); err != nil {
			return nil, err
		}
		if _, err := index.Write(entry.SHA256[:]); err != nil {
			return nil, err
		}
		if err := binary.Write(&index, binary.LittleEndian, uint32(len(entry.Chunks))); err != nil {
			return nil, err
		}
		for _, chunk := range entry.Chunks {
			if err := binary.Write(&index, binary.LittleEndian, chunk.Offset); err != nil {
				return nil, err
			}
			if err := binary.Write(&index, binary.LittleEndian, chunk.CompressedSize); err != nil {
				return nil, err
			}
			if err := binary.Write(&index, binary.LittleEndian, chunk.UncompressedSize); err != nil {
				return nil, err
			}
			if err := binary.Write(&index, binary.LittleEndian, uint32(chunk.Codec)); err != nil {
				return nil, err
			}
			if err := binary.Write(&index, binary.LittleEndian, chunk.DictionaryID); err != nil {
				return nil, err
			}
			if err := binary.Write(&index, binary.LittleEndian, chunk.Checksum); err != nil {
				return nil, err
			}
		}
	}
	return index.Bytes(), nil
}

func writePAKHeader(output io.Writer, chunkSize, entryCount uint32, indexOffset, indexSize uint64) error {
	header := make([]byte, pakHeaderSize)
	copy(header[:8], pakMagic)
	binary.LittleEndian.PutUint32(header[8:12], pakVersion)
	binary.LittleEndian.PutUint32(header[12:16], pakHeaderSize)
	binary.LittleEndian.PutUint32(header[16:20], chunkSize)
	binary.LittleEndian.PutUint32(header[20:24], entryCount)
	binary.LittleEndian.PutUint64(header[24:32], indexOffset)
	binary.LittleEndian.PutUint64(header[32:40], indexSize)
	_, err := output.Write(header)
	return err
}

func normalizePAKOptions(options PAKOptions) (PAKOptions, error) {
	if options.ChunkSize == 0 {
		options.ChunkSize = DefaultPAKChunkSize
	}
	if options.ZstdLevel == 0 {
		options.ZstdLevel = DefaultPAKZstdLevel
	}
	if options.ChunkSize < 1 || uint64(options.ChunkSize) > uint64(^uint32(0)) {
		return PAKOptions{}, fmt.Errorf("invalid PAK chunk size %d", options.ChunkSize)
	}
	if options.ZstdLevel < -5 || options.ZstdLevel > 22 {
		return PAKOptions{}, fmt.Errorf("invalid PAK Zstd level %d", options.ZstdLevel)
	}
	return options, nil
}

func isPAKRawSource(name string, options PAKOptions) bool {
	extension := strings.ToLower(filepath.Ext(name))
	for _, suffix := range options.RawSuffixes {
		if strings.ToLower(strings.TrimSpace(suffix)) == extension {
			return true
		}
	}
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".mp3", ".ogg", ".mp4", ".webm", ".mkv", ".zip", ".gz", ".7z", ".rar", ".dds", ".ktx", ".ktx2", ".basis", ".astc", ".etc2":
		return true
	default:
		return false
	}
}

func normalizePAKName(name string) (string, error) {
	// PAK indexes normally contain UTF-8 names, but callers may still pass
	// legacy EUC-KR/CP949 bytes read from RSM/RSW resources. Decode at the PAK
	// boundary so direct PAK APIs behave like the existing GRF lookup path.
	name = decodeGRFName([]byte(strings.TrimSpace(name)))
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") || (len(name) > 1 && name[1] == ':') {
		return "", fmt.Errorf("invalid path")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("path escapes pack: %s", name)
		}
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path escapes pack: %s", name)
	}
	if strings.EqualFold(clean, "root") {
		return "", fmt.Errorf("invalid root directory")
	}
	if len(clean) >= len("root/") && strings.EqualFold(clean[:len("root/")], "root/") {
		clean = clean[len("root/"):]
	}
	return clean, nil
}

func pakLookupName(name string) string {
	canonical, err := normalizePAKName(name)
	if err != nil {
		return ""
	}
	return strings.ToLower(canonical)
}

type pakIndexReader struct {
	data []byte
	pos  int
}

func (r *pakIndexReader) remaining() int { return len(r.data) - r.pos }

func (r *pakIndexReader) bytes(count int) ([]byte, error) {
	if count < 0 || r.remaining() < count {
		return nil, io.ErrUnexpectedEOF
	}
	value := r.data[r.pos : r.pos+count]
	r.pos += count
	return value, nil
}

func (r *pakIndexReader) u32() (uint32, error) {
	data, err := r.bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (r *pakIndexReader) u64() (uint64, error) {
	data, err := r.bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (r *pakIndexReader) string() (string, error) {
	length, err := r.u32()
	if err != nil {
		return "", err
	}
	if uint64(length) > uint64(r.remaining()) || uint64(length) > uint64(maxInt()) {
		return "", io.ErrUnexpectedEOF
	}
	data, err := r.bytes(int(length))
	return string(data), err
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
