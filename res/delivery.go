package res

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// DeliveryFormat identifies a format that may be hosted by a mobile asset
// delivery manifest. PAK is Goro's native format; the other formats are kept
// for compatibility with existing Ragnarok patch infrastructure.
type DeliveryFormat string

const (
	DeliveryPAK  DeliveryFormat = "pak"
	DeliveryGRF  DeliveryFormat = "grf"
	DeliveryGPF  DeliveryFormat = "gpf"
	DeliveryTHOR DeliveryFormat = "thor"
	DeliveryRGZ  DeliveryFormat = "rgz"
)

var (
	ErrDeliveryUnsupportedFormat = errors.New("unsupported delivery format")
	ErrDeliveryCorrupt           = errors.New("corrupt delivery artifact")
	ErrDeliveryUnsafePath        = errors.New("unsafe delivery path")
	ErrDeliveryUnsupportedMode   = errors.New("unsupported delivery mode")
	ErrDeliveryNotFound          = errors.New("delivery entry not found")
)

type PatchOperationKind string

const (
	PatchAdd    PatchOperationKind = "add"
	PatchRemove PatchOperationKind = "remove"
)

// PatchOperation is the normalized view shared by all delivery formats.
// Data is only used by writers and is intentionally not serialized into a
// manifest.
type PatchOperation struct {
	Kind   PatchOperationKind `json:"kind"`
	Path   string             `json:"path"`
	Source string             `json:"source,omitempty"`
	Size   int64              `json:"size,omitempty"`
	SHA256 string             `json:"sha256,omitempty"`
	Data   []byte             `json:"-"`
}

type DeliveryOptions struct {
	MaxArchiveBytes   int64
	MaxFileBytes      int64
	Android           bool
	ExpectedTargetGRF string
}

// DeliveryAdapter is the common read/validation surface for hosted release
// artifacts. Direct containers and patch archives both expose their logical
// operations through this interface; callers do not need to know whether the
// artifact is mounted or applied to a release-local overlay.
type DeliveryAdapter interface {
	Format() DeliveryFormat
	Operations(filePath string, options DeliveryOptions) ([]PatchOperation, error)
	Validate(filePath string, options DeliveryOptions) error
}

type deliveryAdapter struct {
	format DeliveryFormat
}

func (a deliveryAdapter) Format() DeliveryFormat { return a.format }

func (a deliveryAdapter) Operations(filePath string, options DeliveryOptions) ([]PatchOperation, error) {
	return ReadDeliveryOperations(filePath, a.format, options)
}

func (a deliveryAdapter) Validate(filePath string, options DeliveryOptions) error {
	_, err := a.Operations(filePath, options)
	return err
}

// NewDeliveryAdapter returns the format-neutral validator/operation reader
// used by release tooling and platform integrations.
func NewDeliveryAdapter(format DeliveryFormat) (DeliveryAdapter, error) {
	if !format.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrDeliveryUnsupportedFormat, format)
	}
	return deliveryAdapter{format: format}, nil
}

// OpenDeliveryAdapter is an explicit alias for callers that use the same
// naming convention as OpenPAK/OpenGRF.
func OpenDeliveryAdapter(format DeliveryFormat) (DeliveryAdapter, error) {
	return NewDeliveryAdapter(format)
}

func DefaultDeliveryOptions() DeliveryOptions {
	return DeliveryOptions{MaxArchiveBytes: 8 << 30, MaxFileBytes: 4 << 30}
}

func normalizeDeliveryOptions(options DeliveryOptions) DeliveryOptions {
	defaults := DefaultDeliveryOptions()
	if options.MaxArchiveBytes == 0 {
		options.MaxArchiveBytes = defaults.MaxArchiveBytes
	}
	if options.MaxFileBytes == 0 {
		options.MaxFileBytes = defaults.MaxFileBytes
	}
	return options
}

func (f DeliveryFormat) Valid() bool {
	switch f {
	case DeliveryPAK, DeliveryGRF, DeliveryGPF, DeliveryTHOR, DeliveryRGZ:
		return true
	default:
		return false
	}
}

func ParseDeliveryFormat(value string) (DeliveryFormat, error) {
	format := DeliveryFormat(strings.ToLower(strings.TrimSpace(value)))
	if !format.Valid() {
		return "", fmt.Errorf("%w: %q", ErrDeliveryUnsupportedFormat, value)
	}
	return format, nil
}

func normalizeDeliveryPath(name string) (string, error) {
	name = strings.Trim(name, "\x00")
	name = decodeGRFName([]byte(name))
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, ".\\")
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") || (len(name) > 1 && name[1] == ':') {
		return "", fmt.Errorf("%w: %q", ErrDeliveryUnsafePath, name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("%w: %q", ErrDeliveryUnsafePath, name)
		}
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q", ErrDeliveryUnsafePath, name)
	}
	if strings.EqualFold(clean, "root") {
		return "", fmt.Errorf("%w: root directory is not a resource", ErrDeliveryUnsafePath)
	}
	if len(clean) >= len("root/") && strings.EqualFold(clean[:len("root/")], "root/") {
		clean = clean[len("root/"):]
	}
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q", ErrDeliveryUnsafePath, name)
	}
	return strings.ToLower(clean), nil
}

func validateDeliveryPath(name string, options DeliveryOptions) (string, error) {
	canonical, err := normalizeDeliveryPath(name)
	if err != nil {
		return "", err
	}
	if options.Android {
		switch strings.ToLower(filepath.Ext(canonical)) {
		case ".exe", ".dll", ".so", ".dylib", ".apk", ".dex", ".jar", ".bat", ".cmd", ".sh":
			return "", fmt.Errorf("%w: executable update %q is not allowed on Android", ErrDeliveryUnsafePath, canonical)
		}
	}
	return canonical, nil
}

func readBoundedFile(filePath string, max int64) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrDeliveryCorrupt, filePath)
	}
	if max > 0 && info.Size() > max {
		return nil, fmt.Errorf("%w: %s is %d bytes, maximum is %d", ErrDeliveryCorrupt, filePath, info.Size(), max)
	}
	return os.ReadFile(filePath)
}

func validateOperation(op PatchOperation, options DeliveryOptions) (PatchOperation, error) {
	canonical, err := validateDeliveryPath(op.Path, options)
	if err != nil {
		return PatchOperation{}, err
	}
	op.Path = canonical
	if op.Kind != PatchAdd && op.Kind != PatchRemove {
		return PatchOperation{}, fmt.Errorf("%w: operation %q", ErrDeliveryCorrupt, op.Kind)
	}
	if op.Kind == PatchRemove {
		op.Data = nil
		op.Size = 0
		op.SHA256 = ""
		return op, nil
	}
	if op.Data == nil && op.Source != "" {
		data, readErr := readBoundedFile(op.Source, options.MaxFileBytes)
		if readErr != nil {
			return PatchOperation{}, readErr
		}
		op.Data = data
	}
	if options.MaxFileBytes > 0 && int64(len(op.Data)) > options.MaxFileBytes {
		return PatchOperation{}, fmt.Errorf("%w: %s exceeds maximum file size", ErrDeliveryCorrupt, op.Path)
	}
	if op.Size != 0 && op.Size != int64(len(op.Data)) {
		return PatchOperation{}, fmt.Errorf("%w: %s declared size %d != %d", ErrDeliveryCorrupt, op.Path, op.Size, len(op.Data))
	}
	op.Size = int64(len(op.Data))
	hash := sha256.Sum256(op.Data)
	if op.SHA256 != "" && !strings.EqualFold(op.SHA256, fmt.Sprintf("%x", hash[:])) {
		return PatchOperation{}, fmt.Errorf("%w: %s hash mismatch", ErrDeliveryCorrupt, op.Path)
	}
	op.SHA256 = fmt.Sprintf("%x", hash[:])
	return op, nil
}

// ReadDeliveryOperations validates an artifact and returns its logical
// additions/removals. It is useful for release diffing and pre-activation
// validation without mutating the active resource root.
func ReadDeliveryOperations(filePath string, format DeliveryFormat, options DeliveryOptions) ([]PatchOperation, error) {
	if !format.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrDeliveryUnsupportedFormat, format)
	}
	options = normalizeDeliveryOptions(options)
	if err := validateDeliveryArchiveSize(filePath, options.MaxArchiveBytes); err != nil {
		return nil, err
	}
	switch format {
	case DeliveryPAK:
		archive, err := OpenPAK(filePath)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		operations := make([]PatchOperation, 0, archive.Count())
		for _, name := range archive.Names() {
			canonical, err := validateDeliveryPath(name, options)
			if err != nil {
				return nil, err
			}
			entry, _ := archive.Entry(name)
			if options.MaxFileBytes > 0 && entry.UncompressedSize > uint64(options.MaxFileBytes) {
				return nil, fmt.Errorf("%w: %s exceeds maximum file size", ErrDeliveryCorrupt, name)
			}
			if _, err := archive.ReadFile(name); err != nil {
				return nil, err
			}
			operations = append(operations, PatchOperation{Kind: PatchAdd, Path: canonical, Size: int64(entry.UncompressedSize), SHA256: fmt.Sprintf("%x", entry.SHA256[:])})
		}
		return operations, nil
	case DeliveryGRF, DeliveryGPF:
		archive, err := OpenGRF(filePath)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		operations := make([]PatchOperation, 0, archive.Count())
		for _, name := range archive.Names() {
			entry, _ := archive.Entry(name)
			if options.MaxFileBytes > 0 && int64(entry.RealSize) > options.MaxFileBytes {
				return nil, fmt.Errorf("%w: %s exceeds maximum file size", ErrDeliveryCorrupt, name)
			}
			data, err := archive.ReadFile(name)
			if err != nil {
				return nil, err
			}
			validated, err := validateOperation(PatchOperation{Kind: PatchAdd, Path: name, Data: data}, options)
			if err != nil {
				return nil, err
			}
			validated.Data = nil
			operations = append(operations, validated)
		}
		return operations, nil
	case DeliveryTHOR:
		archive, err := OpenTHOR(filePath, options)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		return archive.Operations(options)
	case DeliveryRGZ:
		archive, err := OpenRGZ(filePath, options)
		if err != nil {
			return nil, err
		}
		return archive.Operations(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrDeliveryUnsupportedFormat, format)
	}
}

func validateDeliveryArchiveSize(filePath string, maximum int64) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrDeliveryCorrupt, filePath)
	}
	if maximum > 0 && info.Size() > maximum {
		return fmt.Errorf("%w: %s is %d bytes, maximum is %d", ErrDeliveryCorrupt, filePath, info.Size(), maximum)
	}
	return nil
}

type THOR struct {
	path        string
	file        *os.File
	UseGRFMerge bool
	Mode        uint16
	TargetGRF   string
	entries     []thorEntry
	fileSize    int64
}

type thorEntry struct {
	PatchOperation
	offset           uint64
	compressedSize   uint32
	uncompressedSize uint32
}

func OpenTHOR(filePath string, options DeliveryOptions) (*THOR, error) {
	options = normalizeDeliveryOptions(options)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if options.MaxArchiveBytes > 0 && info.Size() > options.MaxArchiveBytes {
		file.Close()
		return nil, fmt.Errorf("%w: THOR exceeds maximum archive size", ErrDeliveryCorrupt)
	}
	thor := &THOR{path: filePath, file: file, fileSize: info.Size()}
	if err := thor.load(options); err != nil {
		file.Close()
		return nil, err
	}
	return thor, nil
}

func (t *THOR) Path() string { return t.path }

func (t *THOR) Close() error {
	if t == nil || t.file == nil {
		return nil
	}
	return t.file.Close()
}

func (t *THOR) Count() int { return len(t.entries) }

func (t *THOR) Operations(options DeliveryOptions) ([]PatchOperation, error) {
	operations := make([]PatchOperation, 0, len(t.entries))
	for _, entry := range t.entries {
		if entry.Kind == PatchRemove {
			operations = append(operations, PatchOperation{Kind: PatchRemove, Path: entry.Path})
			continue
		}
		data, err := t.readEntry(entry, options)
		if err != nil {
			return nil, err
		}
		validated, err := validateOperation(PatchOperation{Kind: PatchAdd, Path: entry.Path, Data: data, Size: int64(entry.uncompressedSize)}, options)
		if err != nil {
			return nil, err
		}
		validated.Data = nil
		operations = append(operations, validated)
	}
	return operations, nil
}

func (t *THOR) ReadFile(name string, options DeliveryOptions) ([]byte, error) {
	canonical, err := validateDeliveryPath(name, options)
	if err != nil {
		return nil, err
	}
	for _, entry := range t.entries {
		if entry.Path == canonical {
			if entry.Kind == PatchRemove {
				return nil, ErrDeliveryNotFound
			}
			return t.readEntry(entry, options)
		}
	}
	return nil, ErrDeliveryNotFound
}

func (t *THOR) load(options DeliveryOptions) error {
	const magic = "ASSF (C) 2007 Aeomin DEV"
	if t.fileSize < int64(len(magic)+1+4+2+1) {
		return fmt.Errorf("%w: THOR header is truncated", ErrDeliveryCorrupt)
	}
	position := int64(0)
	readBytes := func(count int64) ([]byte, error) {
		if count < 0 || count > t.fileSize-position {
			return nil, io.ErrUnexpectedEOF
		}
		data := make([]byte, int(count))
		if _, err := t.file.ReadAt(data, position); err != nil {
			return nil, err
		}
		position += count
		return data, nil
	}
	magicBytes, err := readBytes(int64(len(magic)))
	if err != nil || string(magicBytes) != magic {
		return fmt.Errorf("%w: invalid THOR signature", ErrDeliveryCorrupt)
	}
	merge, err := readBytes(1)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeliveryCorrupt, err)
	}
	t.UseGRFMerge = merge[0] != 0
	countBytes, err := readBytes(4)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeliveryCorrupt, err)
	}
	count := binary.LittleEndian.Uint32(countBytes)
	if uint64(count) > uint64(t.fileSize) {
		return fmt.Errorf("%w: THOR entry count is unreasonable", ErrDeliveryCorrupt)
	}
	modeBytes, err := readBytes(2)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeliveryCorrupt, err)
	}
	t.Mode = binary.LittleEndian.Uint16(modeBytes)
	if t.Mode != 0x21 && t.Mode != 0x30 {
		return fmt.Errorf("%w: THOR mode 0x%x", ErrDeliveryUnsupportedMode, t.Mode)
	}
	targetLengthBytes, err := readBytes(1)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeliveryCorrupt, err)
	}
	target, err := readBytes(int64(targetLengthBytes[0]))
	if err != nil {
		return fmt.Errorf("%w: target GRF: %v", ErrDeliveryCorrupt, err)
	}
	t.TargetGRF = strings.TrimRight(string(target), "\x00")
	if options.ExpectedTargetGRF != "" && t.TargetGRF != "" && !strings.EqualFold(filepath.Base(t.TargetGRF), filepath.Base(options.ExpectedTargetGRF)) {
		return fmt.Errorf("%w: target GRF %q does not match %q", ErrDeliveryCorrupt, t.TargetGRF, options.ExpectedTargetGRF)
	}
	if t.Mode == 0x21 {
		if count != 1 {
			return fmt.Errorf("%w: single-file THOR has %d entries", ErrDeliveryCorrupt, count)
		}
		reserved, err := readBytes(1)
		if err != nil {
			return fmt.Errorf("%w: single-file reserved byte: %v", ErrDeliveryCorrupt, err)
		}
		if reserved[0] != 0 {
			return fmt.Errorf("%w: single-file reserved byte is 0x%02x", ErrDeliveryCorrupt, reserved[0])
		}
		return t.loadSingle(readBytes, func() int64 { return position }, options)
	}
	desc, err := readBytes(8)
	if err != nil {
		return fmt.Errorf("%w: table descriptor: %v", ErrDeliveryCorrupt, err)
	}
	tableSize := uint64(binary.LittleEndian.Uint32(desc[:4]))
	tableOffset := uint64(binary.LittleEndian.Uint32(desc[4:8]))
	if tableOffset < uint64(position) || tableOffset > uint64(t.fileSize) || tableSize > uint64(t.fileSize)-tableOffset {
		return fmt.Errorf("%w: THOR table is outside the file", ErrDeliveryCorrupt)
	}
	if options.MaxArchiveBytes > 0 && tableSize > uint64(options.MaxArchiveBytes) {
		return fmt.Errorf("%w: THOR table is too large", ErrDeliveryCorrupt)
	}
	compressed := make([]byte, int(tableSize))
	if _, err := t.file.ReadAt(compressed, int64(tableOffset)); err != nil {
		return err
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("%w: THOR table zlib: %v", ErrDeliveryCorrupt, err)
	}
	table, err := io.ReadAll(io.LimitReader(reader, maxRead(options.MaxArchiveBytes, 64<<20)+1))
	closeErr := reader.Close()
	if err != nil {
		return fmt.Errorf("%w: THOR table decode: %v", ErrDeliveryCorrupt, err)
	}
	if closeErr != nil {
		return fmt.Errorf("%w: THOR table close: %v", ErrDeliveryCorrupt, closeErr)
	}
	if options.MaxArchiveBytes > 0 && int64(len(table)) > options.MaxArchiveBytes {
		return fmt.Errorf("%w: THOR table is too large", ErrDeliveryCorrupt)
	}
	return t.parseMultipleTable(table, count, tableOffset, options)
}

func (t *THOR) loadSingle(readBytes func(int64) ([]byte, error), currentPosition func() int64, options DeliveryOptions) error {
	compressedSizeBytes, err := readBytes(4)
	if err != nil {
		return fmt.Errorf("%w: single-file compressed size: %v", ErrDeliveryCorrupt, err)
	}
	uncompressedSizeBytes, err := readBytes(4)
	if err != nil {
		return fmt.Errorf("%w: single-file size: %v", ErrDeliveryCorrupt, err)
	}
	nameLengthBytes, err := readBytes(1)
	if err != nil {
		return fmt.Errorf("%w: single-file name length: %v", ErrDeliveryCorrupt, err)
	}
	nameBytes, err := readBytes(int64(nameLengthBytes[0]))
	if err != nil {
		return fmt.Errorf("%w: single-file name: %v", ErrDeliveryCorrupt, err)
	}
	name, err := validateDeliveryPath(string(nameBytes), options)
	if err != nil {
		return err
	}
	compressedSize := binary.LittleEndian.Uint32(compressedSizeBytes)
	uncompressedSize := binary.LittleEndian.Uint32(uncompressedSizeBytes)
	if options.MaxFileBytes > 0 && int64(uncompressedSize) > options.MaxFileBytes {
		return fmt.Errorf("%w: %s exceeds maximum file size", ErrDeliveryCorrupt, name)
	}
	dataOffset := currentPosition()
	if uint64(dataOffset) > uint64(t.fileSize) || uint64(compressedSize) > uint64(t.fileSize)-uint64(dataOffset) {
		return fmt.Errorf("%w: single-file data is outside the file", ErrDeliveryCorrupt)
	}
	t.entries = []thorEntry{{PatchOperation: PatchOperation{Kind: PatchAdd, Path: name, Size: int64(uncompressedSize)}, offset: uint64(dataOffset), compressedSize: compressedSize, uncompressedSize: uncompressedSize}}
	return nil
}

func (t *THOR) parseMultipleTable(table []byte, count uint32, tableOffset uint64, options DeliveryOptions) error {
	position := 0
	entries := make([]thorEntry, 0, count)
	seen := make(map[string]struct{}, count)
	for index := uint32(0); index < count; index++ {
		if position >= len(table) {
			return fmt.Errorf("%w: THOR table ended at entry %d", ErrDeliveryCorrupt, index)
		}
		nameLength := int(table[position])
		position++
		if len(table)-position < nameLength+1 {
			return fmt.Errorf("%w: THOR entry %d is truncated", ErrDeliveryCorrupt, index)
		}
		name, err := validateDeliveryPath(string(table[position:position+nameLength]), options)
		if err != nil {
			return err
		}
		position += nameLength
		flags := table[position]
		position++
		if flags&0x01 != 0 {
			if flags != 0x01 {
				return fmt.Errorf("%w: THOR entry %d flags 0x%x", ErrDeliveryCorrupt, index, flags)
			}
			entries = append(entries, thorEntry{PatchOperation: PatchOperation{Kind: PatchRemove, Path: name}})
		} else {
			if len(table)-position < 12 {
				return fmt.Errorf("%w: THOR entry %d metadata is truncated", ErrDeliveryCorrupt, index)
			}
			offset := uint64(binary.LittleEndian.Uint32(table[position : position+4]))
			compressedSize := binary.LittleEndian.Uint32(table[position+4 : position+8])
			uncompressedSize := binary.LittleEndian.Uint32(table[position+8 : position+12])
			position += 12
			if offset < 1 || offset > uint64(t.fileSize) || uint64(compressedSize) > uint64(t.fileSize)-offset || offset+uint64(compressedSize) > tableOffset {
				return fmt.Errorf("%w: THOR entry %s data is outside the data section", ErrDeliveryCorrupt, name)
			}
			if options.MaxFileBytes > 0 && int64(uncompressedSize) > options.MaxFileBytes {
				return fmt.Errorf("%w: %s exceeds maximum file size", ErrDeliveryCorrupt, name)
			}
			entries = append(entries, thorEntry{PatchOperation: PatchOperation{Kind: PatchAdd, Path: name, Size: int64(uncompressedSize)}, offset: offset, compressedSize: compressedSize, uncompressedSize: uncompressedSize})
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%w: duplicate THOR path %s", ErrDeliveryCorrupt, name)
		}
		seen[name] = struct{}{}
	}
	if position != len(table) {
		return fmt.Errorf("%w: trailing THOR table bytes", ErrDeliveryCorrupt)
	}
	t.entries = entries
	return nil
}

func (t *THOR) readEntry(entry thorEntry, options DeliveryOptions) ([]byte, error) {
	if entry.Kind == PatchRemove {
		return nil, ErrDeliveryNotFound
	}
	compressed := make([]byte, int(entry.compressedSize))
	if _, err := t.file.ReadAt(compressed, int64(entry.offset)); err != nil {
		return nil, err
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("%w: THOR entry %s zlib: %v", ErrDeliveryCorrupt, entry.Path, err)
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(entry.uncompressedSize)+1))
	closeErr := reader.Close()
	if err != nil || closeErr != nil {
		return nil, fmt.Errorf("%w: THOR entry %s decode: %v", ErrDeliveryCorrupt, entry.Path, firstError(err, closeErr))
	}
	if len(data) != int(entry.uncompressedSize) {
		return nil, fmt.Errorf("%w: THOR entry %s size %d != %d", ErrDeliveryCorrupt, entry.Path, len(data), entry.uncompressedSize)
	}
	return data, nil
}

type RGZ struct {
	path       string
	operations []PatchOperation
}

func OpenRGZ(filePath string, options DeliveryOptions) (*RGZ, error) {
	options = normalizeDeliveryOptions(options)
	data, err := readBoundedFile(filePath, options.MaxArchiveBytes)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: RGZ gzip: %v", ErrDeliveryCorrupt, err)
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, maxRead(options.MaxArchiveBytes, 64<<20)+1))
	closeErr := reader.Close()
	if err != nil || closeErr != nil {
		return nil, fmt.Errorf("%w: RGZ decode: %v", ErrDeliveryCorrupt, firstError(err, closeErr))
	}
	if options.MaxArchiveBytes > 0 && int64(len(decoded)) > options.MaxArchiveBytes {
		return nil, fmt.Errorf("%w: RGZ decoded data is too large", ErrDeliveryCorrupt)
	}
	operations, err := parseRGZ(decoded, options)
	if err != nil {
		return nil, err
	}
	return &RGZ{path: filePath, operations: operations}, nil
}

func (r *RGZ) Path() string { return r.path }
func (r *RGZ) Close() error { return nil }
func (r *RGZ) Operations() []PatchOperation {
	return append([]PatchOperation(nil), r.operations...)
}

func parseRGZ(data []byte, options DeliveryOptions) ([]PatchOperation, error) {
	position := 0
	ended := false
	operations := make([]PatchOperation, 0)
	seen := map[string]struct{}{}
	for position < len(data) {
		entryType := data[position]
		position++
		if position >= len(data) {
			return nil, fmt.Errorf("%w: RGZ name length is truncated", ErrDeliveryCorrupt)
		}
		nameLength := int(data[position])
		position++
		if len(data)-position < nameLength {
			return nil, fmt.Errorf("%w: RGZ name is truncated", ErrDeliveryCorrupt)
		}
		nameBytes := data[position : position+nameLength]
		position += nameLength
		name := strings.TrimRight(string(nameBytes), "\x00")
		if entryType == 'e' {
			if !strings.EqualFold(name, "end") {
				return nil, fmt.Errorf("%w: RGZ end entry name is %q", ErrDeliveryCorrupt, name)
			}
			ended = true
			if position != len(data) {
				return nil, fmt.Errorf("%w: RGZ has bytes after end entry", ErrDeliveryCorrupt)
			}
			break
		}
		canonical, err := validateDeliveryPath(name, options)
		if err != nil {
			return nil, err
		}
		switch entryType {
		case 'd':
			// Directory records establish paths for legacy patchers. The native
			// overlay can create parents as needed, so they are not resources.
		case 'f':
			if len(data)-position < 4 {
				return nil, fmt.Errorf("%w: RGZ file size is truncated", ErrDeliveryCorrupt)
			}
			size := binary.LittleEndian.Uint32(data[position : position+4])
			position += 4
			if options.MaxFileBytes > 0 && int64(size) > options.MaxFileBytes {
				return nil, fmt.Errorf("%w: RGZ file %s exceeds maximum file size", ErrDeliveryCorrupt, canonical)
			}
			if uint64(size) > uint64(len(data)-position) {
				return nil, fmt.Errorf("%w: RGZ file %s contents are truncated", ErrDeliveryCorrupt, canonical)
			}
			body := append([]byte(nil), data[position:position+int(size)]...)
			position += int(size)
			operation, err := validateOperation(PatchOperation{Kind: PatchAdd, Path: canonical, Data: body}, options)
			if err != nil {
				return nil, err
			}
			if _, duplicate := seen[canonical]; duplicate {
				return nil, fmt.Errorf("%w: duplicate RGZ path %s", ErrDeliveryCorrupt, canonical)
			}
			seen[canonical] = struct{}{}
			operation.Data = nil
			operations = append(operations, operation)
		default:
			return nil, fmt.Errorf("%w: RGZ entry type %q", ErrDeliveryUnsupportedMode, entryType)
		}
	}
	if !ended {
		return nil, fmt.Errorf("%w: RGZ is missing final end entry", ErrDeliveryCorrupt)
	}
	return operations, nil
}

func ApplyTHOR(filePath, destination string, options DeliveryOptions) ([]PatchOperation, []string, error) {
	archive, err := OpenTHOR(filePath, options)
	if err != nil {
		return nil, nil, err
	}
	defer archive.Close()
	if err := prepareOverlayDirectory(destination); err != nil {
		return nil, nil, err
	}
	operations := make([]PatchOperation, 0, archive.Count())
	var tombstones []string
	for _, entry := range archive.entries {
		if entry.Kind == PatchRemove {
			tombstones = append(tombstones, entry.Path)
			operations = append(operations, PatchOperation{Kind: PatchRemove, Path: entry.Path})
			continue
		}
		data, err := archive.readEntry(entry, options)
		if err != nil {
			return nil, nil, err
		}
		operation, err := validateOperation(PatchOperation{Kind: PatchAdd, Path: entry.Path, Data: data, Size: int64(entry.uncompressedSize)}, options)
		if err != nil {
			return nil, nil, err
		}
		if err := writeOverlayFile(destination, operation.Path, operation.Data); err != nil {
			return nil, nil, err
		}
		operation.Data = nil
		operations = append(operations, operation)
	}
	sort.Strings(tombstones)
	return operations, tombstones, nil
}

func ApplyRGZ(filePath, destination string, options DeliveryOptions) ([]PatchOperation, error) {
	archive, err := OpenRGZ(filePath, options)
	if err != nil {
		return nil, err
	}
	if err := prepareOverlayDirectory(destination); err != nil {
		return nil, err
	}
	data, err := readBoundedFile(filePath, options.MaxArchiveBytes)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	decoded, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil || closeErr != nil {
		return nil, firstError(err, closeErr)
	}
	// Reparse with data attached so the writer does not expose a second public
	// representation of RGZ's private file records.
	position := 0
	for position < len(decoded) {
		entryType := decoded[position]
		position++
		nameLength := int(decoded[position])
		position++
		name := strings.TrimRight(string(decoded[position:position+nameLength]), "\x00")
		position += nameLength
		if entryType == 'e' {
			break
		}
		if entryType == 'f' {
			size := int(binary.LittleEndian.Uint32(decoded[position : position+4]))
			position += 4
			canonical, err := validateDeliveryPath(name, options)
			if err != nil {
				return nil, err
			}
			if err := writeOverlayFile(destination, canonical, decoded[position:position+size]); err != nil {
				return nil, err
			}
			position += size
		}
	}
	return archive.Operations(), nil
}

func prepareOverlayDirectory(destination string) error {
	if destination == "" {
		return fmt.Errorf("%w: empty overlay directory", ErrDeliveryUnsafePath)
	}
	return os.MkdirAll(destination, 0o755)
}

func writeOverlayFile(root, name string, data []byte) error {
	canonical, err := normalizeDeliveryPath(name)
	if err != nil {
		return err
	}
	target := filepath.Join(root, filepath.FromSlash(canonical))
	if err := ensureNoSymlinkParents(root, filepath.Dir(target)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: overlay target is a symlink: %s", ErrDeliveryUnsafePath, canonical)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".goro-overlay-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func ensureNoSymlinkParents(root, directory string) error {
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: overlay parent is a symlink: %s", ErrDeliveryUnsafePath, current)
		}
	}
	return nil
}

type THORPackOptions struct {
	UseGRFMerge bool
	Mode        uint16
	TargetGRF   string
}

func PackTHOR(outputPath string, operations []PatchOperation, options THORPackOptions) error {
	if options.Mode == 0 {
		options.Mode = 0x30
	}
	if options.Mode != 0x30 && options.Mode != 0x21 {
		return fmt.Errorf("%w: THOR mode 0x%x", ErrDeliveryUnsupportedMode, options.Mode)
	}
	if options.Mode == 0x21 && len(operations) != 1 {
		return fmt.Errorf("%w: single-file THOR requires one operation", ErrDeliveryCorrupt)
	}
	validated := make([]PatchOperation, 0, len(operations))
	seen := map[string]struct{}{}
	for _, original := range operations {
		operation, err := validateOperation(original, DefaultDeliveryOptions())
		if err != nil {
			return err
		}
		if _, exists := seen[operation.Path]; exists {
			return fmt.Errorf("%w: duplicate THOR path %s", ErrDeliveryCorrupt, operation.Path)
		}
		seen[operation.Path] = struct{}{}
		if options.UseGRFMerge {
			operation.Path = "root/" + operation.Path
		}
		validated = append(validated, operation)
	}
	sort.Slice(validated, func(i, j int) bool { return validated[i].Path < validated[j].Path })
	if options.Mode == 0x21 {
		return writeTHORSingle(outputPath, validated[0], options)
	}
	return writeTHORMultiple(outputPath, validated, options)
}

func writeTHORSingle(outputPath string, operation PatchOperation, options THORPackOptions) error {
	if operation.Kind == PatchRemove {
		return fmt.Errorf("%w: single-file THOR cannot encode removals", ErrDeliveryUnsupportedMode)
	}
	compressed, err := zlibData(operation.Data)
	if err != nil {
		return err
	}
	name := encodeDeliveryPathBytes(operation.Path)
	if len(name) > 255 {
		return fmt.Errorf("%w: THOR filename too long", ErrDeliveryCorrupt)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeTHORHeader(file, options, 1); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(len(compressed))); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(len(operation.Data))); err != nil {
		return err
	}
	if _, err := file.Write([]byte{byte(len(name))}); err != nil {
		return err
	}
	if _, err := file.Write(name); err != nil {
		return err
	}
	if _, err = file.Write(compressed); err != nil {
		return err
	}
	_, err = file.Write([]byte("SEALED!"))
	return err
}

func writeTHORMultiple(outputPath string, operations []PatchOperation, options THORPackOptions) error {
	data := bytes.NewBuffer(nil)
	entries := make([]thorEntry, 0, len(operations))
	// The writer's header has a variable target name. Calculate the absolute
	// payload offset before encoding table records.
	headerSize := int64(len("ASSF (C) 2007 Aeomin DEV") + 1 + 4 + 2 + 1 + len([]byte(options.TargetGRF)) + 8)
	for _, operation := range operations {
		name := encodeDeliveryPathBytes(operation.Path)
		if len(name) > 255 {
			return fmt.Errorf("%w: THOR filename too long", ErrDeliveryCorrupt)
		}
		entry := thorEntry{PatchOperation: operation}
		if operation.Kind == PatchAdd {
			compressed, err := zlibData(operation.Data)
			if err != nil {
				return err
			}
			entry.offset = uint64(headerSize + int64(data.Len()))
			entry.compressedSize = uint32(len(compressed))
			entry.uncompressedSize = uint32(len(operation.Data))
			if _, err := data.Write(compressed); err != nil {
				return err
			}
		}
		entries = append(entries, entry)
	}
	table := bytes.NewBuffer(nil)
	for _, entry := range entries {
		name := encodeDeliveryPathBytes(entry.Path)
		table.WriteByte(byte(len(name)))
		table.Write(name)
		if entry.Kind == PatchRemove {
			table.WriteByte(0x01)
			continue
		}
		table.WriteByte(0)
		binary.Write(table, binary.LittleEndian, uint32(entry.offset))
		binary.Write(table, binary.LittleEndian, entry.compressedSize)
		binary.Write(table, binary.LittleEndian, entry.uncompressedSize)
	}
	compressedTable, err := zlibData(table.Bytes())
	if err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeTHORHeader(file, options, uint32(len(entries))); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(len(compressedTable))); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(headerSize+int64(data.Len()))); err != nil {
		return err
	}
	if _, err := file.Write(data.Bytes()); err != nil {
		return err
	}
	_, err = file.Write(compressedTable)
	return err
}

func writeTHORHeader(file *os.File, options THORPackOptions, count uint32) error {
	if _, err := file.Write([]byte("ASSF (C) 2007 Aeomin DEV")); err != nil {
		return err
	}
	merge := byte(0)
	if options.UseGRFMerge {
		merge = 1
	}
	if _, err := file.Write([]byte{merge}); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, count); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, options.Mode); err != nil {
		return err
	}
	target := []byte(options.TargetGRF)
	if len(target) > 255 {
		return fmt.Errorf("%w: target GRF name too long", ErrDeliveryCorrupt)
	}
	if _, err := file.Write([]byte{byte(len(target))}); err != nil {
		return err
	}
	if _, err := file.Write(target); err != nil {
		return err
	}
	if options.Mode == 0x21 {
		_, err := file.Write([]byte{0})
		return err
	}
	return nil
}

func PackRGZ(outputPath string, operations []PatchOperation) error {
	validated := make([]PatchOperation, 0, len(operations))
	for _, original := range operations {
		if original.Kind == PatchRemove {
			return fmt.Errorf("%w: RGZ has no deletion record for %s", ErrDeliveryUnsupportedMode, original.Path)
		}
		operation, err := validateOperation(original, DefaultDeliveryOptions())
		if err != nil {
			return err
		}
		validated = append(validated, operation)
	}
	sort.Slice(validated, func(i, j int) bool { return validated[i].Path < validated[j].Path })
	var raw bytes.Buffer
	directories := map[string]struct{}{}
	for _, operation := range validated {
		parts := strings.Split(operation.Path, "/")
		for index := 1; index < len(parts); index++ {
			directory := strings.Join(parts[:index], "/")
			if _, exists := directories[directory]; exists {
				continue
			}
			directories[directory] = struct{}{}
			if err := writeRGZEntry(&raw, 'd', directory, nil); err != nil {
				return err
			}
		}
		if err := writeRGZEntry(&raw, 'f', operation.Path, operation.Data); err != nil {
			return err
		}
	}
	if err := writeRGZEntry(&raw, 'e', "end", nil); err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	writer := gzip.NewWriter(file)
	_, writeErr := writer.Write(raw.Bytes())
	closeWriterErr := writer.Close()
	closeFileErr := file.Close()
	return firstError(writeErr, closeWriterErr, closeFileErr)
}

func writeRGZEntry(raw *bytes.Buffer, entryType byte, name string, data []byte) error {
	nameBytes := append(encodeDeliveryPathBytes(name), 0)
	if len(nameBytes) > 255 {
		return fmt.Errorf("%w: RGZ name is too long: %s", ErrDeliveryCorrupt, name)
	}
	raw.WriteByte(entryType)
	raw.WriteByte(byte(len(nameBytes)))
	raw.Write(nameBytes)
	if entryType == 'f' {
		binary.Write(raw, binary.LittleEndian, uint32(len(data)))
		raw.Write(data)
	}
	return nil
}

func encodeDeliveryPathBytes(name string) []byte {
	encoded, err := encodeGRFTableName(name)
	if err == nil {
		return []byte(encoded)
	}
	return []byte(name)
}

func zlibData(data []byte) ([]byte, error) {
	var output bytes.Buffer
	writer := zlib.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func maxRead(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
