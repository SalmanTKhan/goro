package res

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
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

type GRFPackStats struct {
	Files           int
	Bytes           int64
	CompressedBytes int64
	ArchiveBytes    int64
}

// GRFPackSource is one logical resource supplied to PackGRFEntries. Open is
// called once and may stream directly from a source archive or loose file.
type GRFPackSource struct {
	Name string
	Size int64
	Open func() (io.ReadCloser, error)
}

type grfPackEntry struct {
	name        string
	packedSize  uint32
	alignedSize uint32
	realSize    uint32
	offset      uint32
}

func PackGRF(outputPath, root string) (GRFPackStats, error) {
	files, err := collectGRFFiles(root)
	if err != nil {
		return GRFPackStats{}, err
	}
	sources := make([]GRFPackSource, 0, len(files))
	for _, grfName := range files {
		name := grfName
		filePath := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Stat(filePath)
		if err != nil {
			return GRFPackStats{}, err
		}
		sources = append(sources, GRFPackSource{
			Name: name,
			Size: info.Size(),
			Open: func() (io.ReadCloser, error) { return os.Open(filePath) },
		})
	}
	return PackGRFEntries(outputPath, sources)
}

// PackGRFEntries writes a deterministic GRF from streamed logical resources.
// It does not create an extracted staging directory and only holds the
// compressed table in memory.
func PackGRFEntries(outputPath string, sources []GRFPackSource) (GRFPackStats, error) {
	sources = append([]GRFPackSource(nil), sources...)
	sort.SliceStable(sources, func(i, j int) bool {
		left := strings.ToLower(filepath.ToSlash(sources[i].Name))
		right := strings.ToLower(filepath.ToSlash(sources[j].Name))
		if left == right {
			return sources[i].Name < sources[j].Name
		}
		return left < right
	})

	out, err := os.Create(outputPath)
	if err != nil {
		return GRFPackStats{}, err
	}
	closeOutput := true
	defer func() {
		if closeOutput {
			_ = out.Close()
		}
	}()

	header := make([]byte, grfHeaderSize)
	copy(header[:15], []byte("Master of Magic"))
	binary.LittleEndian.PutUint32(header[34:38], 0)
	binary.LittleEndian.PutUint32(header[38:42], uint32(len(sources)+7))
	binary.LittleEndian.PutUint32(header[42:46], grfVersion200)
	if _, err := out.Write(header); err != nil {
		return GRFPackStats{}, err
	}

	entries := make([]grfPackEntry, 0, len(sources))
	var stats GRFPackStats
	for _, source := range sources {
		if source.Name == "" || source.Open == nil {
			return GRFPackStats{}, fmt.Errorf("invalid GRF source %q", source.Name)
		}
		reader, err := source.Open()
		if err != nil {
			return GRFPackStats{}, fmt.Errorf("open %s: %w", source.Name, err)
		}
		offset, err := out.Seek(0, io.SeekCurrent)
		if err != nil {
			_ = reader.Close()
			return GRFPackStats{}, err
		}
		if offset < grfHeaderSize || offset-grfHeaderSize > int64(^uint32(0)) {
			_ = reader.Close()
			return GRFPackStats{}, fmt.Errorf("%s: GRF offset overflow", source.Name)
		}
		compressedWriter := zlib.NewWriter(out)
		copied, copyErr := io.Copy(compressedWriter, reader)
		closeReaderErr := reader.Close()
		closeCompressedErr := compressedWriter.Close()
		if copyErr != nil {
			return GRFPackStats{}, fmt.Errorf("compress %s: %w", source.Name, copyErr)
		}
		if closeReaderErr != nil {
			return GRFPackStats{}, fmt.Errorf("close %s: %w", source.Name, closeReaderErr)
		}
		if closeCompressedErr != nil {
			return GRFPackStats{}, fmt.Errorf("finish %s: %w", source.Name, closeCompressedErr)
		}
		if source.Size > 0 && copied != source.Size {
			return GRFPackStats{}, fmt.Errorf("%s: streamed size %d does not match declared size %d", source.Name, copied, source.Size)
		}
		end, err := out.Seek(0, io.SeekCurrent)
		if err != nil {
			return GRFPackStats{}, err
		}
		compressedSize := end - offset
		entries = append(entries, grfPackEntry{
			name:        filepath.ToSlash(source.Name),
			packedSize:  uint32(compressedSize),
			alignedSize: uint32(compressedSize),
			realSize:    uint32(copied),
			offset:      uint32(offset - grfHeaderSize),
		})
		stats.Files++
		stats.Bytes += copied
		stats.CompressedBytes += compressedSize
	}

	table, err := buildGRFTable(entries)
	if err != nil {
		return GRFPackStats{}, err
	}
	compressedTable, err := zlibCompress(table)
	if err != nil {
		return GRFPackStats{}, err
	}
	tableOffset, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return GRFPackStats{}, err
	}
	if tableOffset-grfHeaderSize > int64(^uint32(0)) {
		return GRFPackStats{}, fmt.Errorf("GRF table offset overflow")
	}
	if err := binary.Write(out, binary.LittleEndian, uint32(len(compressedTable))); err != nil {
		return GRFPackStats{}, err
	}
	if err := binary.Write(out, binary.LittleEndian, uint32(len(table))); err != nil {
		return GRFPackStats{}, err
	}
	if _, err := out.Write(compressedTable); err != nil {
		return GRFPackStats{}, err
	}
	if _, err := out.Seek(30, io.SeekStart); err != nil {
		return GRFPackStats{}, err
	}
	if err := binary.Write(out, binary.LittleEndian, uint32(tableOffset-grfHeaderSize)); err != nil {
		return GRFPackStats{}, err
	}
	if err := out.Close(); err != nil {
		return GRFPackStats{}, err
	}
	closeOutput = false
	if info, err := os.Stat(outputPath); err == nil {
		stats.ArchiveBytes = info.Size()
	}
	return stats, nil
}

// EstimateGRFEntries performs the same per-resource zlib pass as the writer
// but sends compressed bytes to a counting sink instead of a file. It is used
// by the asset editor for an exact estimate of the current GRF format.
func EstimateGRFEntries(sources []GRFPackSource) (GRFPackStats, error) {
	sources = append([]GRFPackSource(nil), sources...)
	sort.SliceStable(sources, func(i, j int) bool {
		left := strings.ToLower(filepath.ToSlash(sources[i].Name))
		right := strings.ToLower(filepath.ToSlash(sources[j].Name))
		if left == right {
			return sources[i].Name < sources[j].Name
		}
		return left < right
	})
	entries := make([]grfPackEntry, 0, len(sources))
	var stats GRFPackStats
	offset := int64(grfHeaderSize)
	for _, source := range sources {
		if source.Name == "" || source.Open == nil {
			return GRFPackStats{}, fmt.Errorf("invalid GRF source %q", source.Name)
		}
		reader, err := source.Open()
		if err != nil {
			return GRFPackStats{}, fmt.Errorf("open %s: %w", source.Name, err)
		}
		counter := &countingWriter{}
		compressedWriter := zlib.NewWriter(counter)
		copied, copyErr := io.Copy(compressedWriter, reader)
		closeReaderErr := reader.Close()
		closeCompressedErr := compressedWriter.Close()
		if copyErr != nil {
			return GRFPackStats{}, fmt.Errorf("compress %s: %w", source.Name, copyErr)
		}
		if closeReaderErr != nil {
			return GRFPackStats{}, fmt.Errorf("close %s: %w", source.Name, closeReaderErr)
		}
		if closeCompressedErr != nil {
			return GRFPackStats{}, fmt.Errorf("finish %s: %w", source.Name, closeCompressedErr)
		}
		if source.Size > 0 && copied != source.Size {
			return GRFPackStats{}, fmt.Errorf("%s: streamed size %d does not match declared size %d", source.Name, copied, source.Size)
		}
		entries = append(entries, grfPackEntry{name: filepath.ToSlash(source.Name), packedSize: uint32(counter.count), alignedSize: uint32(counter.count), realSize: uint32(copied), offset: uint32(offset - grfHeaderSize)})
		stats.Files++
		stats.Bytes += copied
		stats.CompressedBytes += counter.count
		offset += counter.count
	}
	table, err := buildGRFTable(entries)
	if err != nil {
		return GRFPackStats{}, err
	}
	compressedTable, err := zlibCompress(table)
	if err != nil {
		return GRFPackStats{}, err
	}
	stats.ArchiveBytes = offset + 8 + int64(len(compressedTable))
	return stats, nil
}

type countingWriter struct{ count int64 }

func (w *countingWriter) Write(data []byte) (int, error) {
	w.count += int64(len(data))
	return len(data), nil
}

func collectGRFFiles(root string) ([]string, error) {
	var files []string
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func buildGRFTable(entries []grfPackEntry) ([]byte, error) {
	var table bytes.Buffer
	for _, entry := range entries {
		name, err := encodeGRFTableName(entry.name)
		if err != nil {
			return nil, err
		}
		table.WriteString(name)
		table.WriteByte(0)
		writeU32Buffer(&table, entry.packedSize)
		writeU32Buffer(&table, entry.alignedSize)
		writeU32Buffer(&table, entry.realSize)
		table.WriteByte(0x01)
		writeU32Buffer(&table, entry.offset)
	}
	return table.Bytes(), nil
}

func encodeGRFTableName(name string) (string, error) {
	name = strings.ReplaceAll(filepath.ToSlash(name), "/", "\\")
	encoded, _, err := transform.String(korean.EUCKR.NewEncoder(), name)
	if err != nil {
		// Some modern client fixtures contain UTF-8 resource names that are
		// outside EUC-KR. The reader already preserves valid UTF-8 GRF names,
		// so retain the UTF-8 spelling instead of making the fixture impossible
		// to build or silently renaming a resource.
		if utf8.ValidString(name) {
			return name, nil
		}
		return "", fmt.Errorf("encode GRF path %q as EUC-KR: %w", name, err)
	}
	return encoded, nil
}

func zlibCompress(data []byte) ([]byte, error) {
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeU32Buffer(buf *bytes.Buffer, value uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], value)
	buf.Write(tmp[:])
}
