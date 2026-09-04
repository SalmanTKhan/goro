//go:build windows

package capture

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MediaFoundationEncoder writes video-only MP4/H.264 using the Windows Sink
// Writer. The implementation deliberately uses the RGB32 input path: the
// encoder owns conversion and can select a hardware MFT when Windows exposes
// one for the installed GPU.
type MediaFoundationEncoder struct {
	mu        sync.Mutex
	options   RecordingOptions
	started   bool
	finalized bool
	stream    uint32
	sink      uintptr
	input     uintptr
	output    uintptr
	attrs     uintptr
	mfReady   bool
}

func NewMediaFoundationEncoder() *MediaFoundationEncoder {
	return &MediaFoundationEncoder{}
}

func (e *MediaFoundationEncoder) Start(options RecordingOptions) error {
	options, err := options.Normalized()
	if err != nil {
		return err
	}
	if options.Container != RecordingMP4 || options.Codec != RecordingH264 {
		return fmt.Errorf("Media Foundation encoder only supports MP4/H.264")
	}
	if strings.TrimSpace(options.Path) == "" {
		return fmt.Errorf("recording path is required")
	}
	if err := os.MkdirAll(filepathDir(options.Path), 0o755); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.mfReady {
		return ErrClosed
	}
	if err := mfStartup(); err != nil {
		return err
	}
	e.mfReady = true
	e.options = options
	return nil
}

func (e *MediaFoundationEncoder) Write(frame Frame) error {
	if e == nil {
		return ErrClosed
	}
	if err := frame.Validate(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.mfReady || e.finalized {
		return ErrClosed
	}
	comInitialized, err := initializeCOM()
	if err != nil {
		return err
	}
	if comInitialized {
		defer uninitializeCOM()
	}
	if !e.started {
		if err := e.startWriter(frame.Width, frame.Height); err != nil {
			e.releaseWriterObjects()
			return err
		}
	}
	return e.writeSample(frame)
}

func (e *MediaFoundationEncoder) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.mfReady {
		return nil
	}
	comInitialized, err := initializeCOM()
	if err != nil {
		return err
	}
	if comInitialized {
		defer uninitializeCOM()
	}
	var result error
	if e.sink != 0 && !e.finalized {
		if err := checkHRESULT("IMFSinkWriter.Finalize", comCall(e.sink, 11)); err != nil {
			result = err
		}
		e.finalized = true
	}
	e.releaseWriterObjects()
	if err := mfShutdown(); result == nil && err != nil {
		result = err
	}
	e.mfReady = false
	e.started = false
	e.finalized = false
	e.stream = 0
	return result
}

func (e *MediaFoundationEncoder) releaseWriterObjects() {
	releaseCOM(e.attrs)
	releaseCOM(e.input)
	releaseCOM(e.output)
	releaseCOM(e.sink)
	e.attrs, e.input, e.output, e.sink = 0, 0, 0, 0
}

func (e *MediaFoundationEncoder) startWriter(width, height int) error {
	path, err := windows.UTF16PtrFromString(e.options.Path)
	if err != nil {
		return err
	}
	attrs, err := newAttributes(2)
	if err != nil {
		return err
	}
	e.attrs = attrs
	if err := setUINT32(attrs, mfReadwriteEnableHardwareTransforms, 1); err != nil {
		releaseCOM(attrs)
		e.attrs = 0
		return err
	}
	if err := setUINT32(attrs, mfSinkWriterDisableThrottling, 1); err != nil {
		releaseCOM(attrs)
		e.attrs = 0
		return err
	}
	var sink uintptr
	hr := mfCreateSinkWriter(path, attrs, &sink)
	if err := checkHRESULT("MFCreateSinkWriterFromURL", hr); err != nil {
		releaseCOM(attrs)
		e.attrs = 0
		return err
	}
	e.sink = sink

	output, err := newMediaType()
	if err != nil {
		return err
	}
	e.output = output
	if err := setGUID(output, mfMTMajorType, mfMediaTypeVideo); err != nil {
		return err
	}
	if err := setGUID(output, mfMTSubtype, mfVideoFormatH264); err != nil {
		return err
	}
	if err := setUINT32(output, mfMTAvgBitrate, uint32(maxInt(width*height*8, 2_000_000))); err != nil { //nolint:gosec // dimensions validated by Frame
		return err
	}
	if err := setUINT64(output, mfMTFrameSize, packUINT32(width, height)); err != nil {
		return err
	}
	if err := setUINT64(output, mfMTFrameRate, packUINT32(e.options.FPS, 1)); err != nil {
		return err
	}
	if err := setUINT64(output, mfMTPixelAspectRatio, packUINT32(1, 1)); err != nil {
		return err
	}
	if err := setUINT32(output, mfMTInterlaceMode, 2); err != nil {
		return err
	}
	input, err := newMediaType()
	if err != nil {
		return err
	}
	e.input = input
	if err := setGUID(input, mfMTMajorType, mfMediaTypeVideo); err != nil {
		return err
	}
	if err := setGUID(input, mfMTSubtype, mfVideoFormatRGB32); err != nil {
		return err
	}
	if err := setUINT64(input, mfMTFrameSize, packUINT32(width, height)); err != nil {
		return err
	}
	if err := setUINT64(input, mfMTFrameRate, packUINT32(e.options.FPS, 1)); err != nil {
		return err
	}
	if err := setUINT64(input, mfMTPixelAspectRatio, packUINT32(1, 1)); err != nil {
		return err
	}
	if err := setUINT32(input, mfMTInterlaceMode, 2); err != nil {
		return err
	}
	var stream uint32
	if err := checkHRESULT("IMFSinkWriter.AddStream", comCall(e.sink, 3, e.output, uintptr(unsafe.Pointer(&stream)))); err != nil {
		return err
	}
	e.stream = stream
	if err := checkHRESULT("IMFSinkWriter.SetInputMediaType", comCall(e.sink, 4, uintptr(stream), e.input, 0)); err != nil {
		return err
	}
	if err := checkHRESULT("IMFSinkWriter.BeginWriting", comCall(e.sink, 5)); err != nil {
		return err
	}
	e.started = true
	return nil
}

func (e *MediaFoundationEncoder) writeSample(frame Frame) error {
	var buffer uintptr
	if err := checkHRESULT("MFCreateMemoryBuffer", mfCreateMemoryBuffer(uint32(frame.Width*frame.Height*4), &buffer)); err != nil { //nolint:gosec // dimensions validated by Frame
		return err
	}
	defer releaseCOM(buffer)
	var data uintptr
	if err := checkHRESULT("IMFMediaBuffer.Lock", comCall(buffer, 3, uintptr(unsafe.Pointer(&data)), 0, 0)); err != nil {
		return err
	}
	if data == 0 {
		_ = comCall(buffer, 4)
		return fmt.Errorf("IMFMediaBuffer.Lock returned a nil data pointer")
	}
	dst := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(nil), data)), frame.Width*frame.Height*4)
	writeTightBGRA(dst, frame)
	if err := checkHRESULT("IMFMediaBuffer.Unlock", comCall(buffer, 4)); err != nil {
		return err
	}
	if err := checkHRESULT("IMFMediaBuffer.SetCurrentLength", comCall(buffer, 6, uintptr(frame.Width*frame.Height*4))); err != nil { //nolint:gosec // dimensions validated by Frame
		return err
	}

	var sample uintptr
	if err := checkHRESULT("MFCreateSample", mfCreateSample(&sample)); err != nil {
		return err
	}
	defer releaseCOM(sample)
	if err := checkHRESULT("IMFSample.AddBuffer", comCall(sample, 42, buffer)); err != nil {
		return err
	}
	if err := checkHRESULT("IMFSample.SetSampleTime", comCall(sample, 36, uintptr(frame.PTS.Microseconds()*10))); err != nil {
		return err
	}
	duration := int64(10_000_000 / e.options.FPS)
	if err := checkHRESULT("IMFSample.SetSampleDuration", comCall(sample, 38, uintptr(duration))); err != nil {
		return err
	}
	return checkHRESULT("IMFSinkWriter.WriteSample", comCall(e.sink, 6, uintptr(e.stream), sample))
}

func writeTightBGRA(dst []byte, frame Frame) {
	row := frame.Width * 4
	for y := 0; y < frame.Height; y++ {
		src := frame.Pixels[y*frame.Stride : y*frame.Stride+row]
		out := dst[y*row : (y+1)*row]
		if frame.PixelFormat == PixelFormatBGRA8 {
			copy(out, src)
			continue
		}
		for i := 0; i < row; i += 4 {
			out[i], out[i+1], out[i+2], out[i+3] = src[i+2], src[i+1], src[i], src[i+3]
		}
	}
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			if i == 0 {
				return path[:1]
			}
			return path[:i]
		}
	}
	return "."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	modOle32                      = windows.NewLazySystemDLL("ole32.dll")
	modMFPlat                     = windows.NewLazySystemDLL("mfplat.dll")
	modMFReadWrite                = windows.NewLazySystemDLL("mfreadwrite.dll")
	procCoInitializeEx            = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize            = modOle32.NewProc("CoUninitialize")
	procMFStartup                 = modMFPlat.NewProc("MFStartup")
	procMFShutdown                = modMFPlat.NewProc("MFShutdown")
	procMFCreateAttributes        = modMFPlat.NewProc("MFCreateAttributes")
	procMFCreateMediaType         = modMFPlat.NewProc("MFCreateMediaType")
	procMFCreateMemoryBuffer      = modMFPlat.NewProc("MFCreateMemoryBuffer")
	procMFCreateSample            = modMFPlat.NewProc("MFCreateSample")
	procMFCreateSinkWriterFromURL = modMFReadWrite.NewProc("MFCreateSinkWriterFromURL")
)

const (
	coinitMultithreaded = 0x0
	mfVersion           = 0x00020070
	rpcEChangedMode     = 0x80010106
)

var (
	mfMediaTypeVideo                    = guid(0x73646976, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71})
	mfVideoFormatRGB32                  = guid(22, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71})
	mfVideoFormatH264                   = guid(0x34363248, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71})
	mfMTMajorType                       = guid(0x48eba18e, 0xf8c9, 0x4687, [8]byte{0xbf, 0x11, 0x0a, 0x74, 0xc9, 0xf9, 0x6a, 0x8f})
	mfMTSubtype                         = guid(0xf7e34c9a, 0x42e8, 0x4714, [8]byte{0xb7, 0x4b, 0xcb, 0x29, 0xd7, 0x2c, 0x35, 0xe5})
	mfMTFrameSize                       = guid(0x1652c33d, 0xd6b2, 0x4012, [8]byte{0xb8, 0x34, 0x72, 0x03, 0x08, 0x49, 0xa3, 0x7d})
	mfMTFrameRate                       = guid(0xc459a2e8, 0x3d2c, 0x4e44, [8]byte{0xb1, 0x32, 0xfe, 0xe5, 0x15, 0x6c, 0x7b, 0xb0})
	mfMTPixelAspectRatio                = guid(0xc6376a1e, 0x8d0a, 0x4027, [8]byte{0xbe, 0x45, 0x6d, 0x9a, 0x0a, 0xd3, 0x9b, 0xb6})
	mfMTInterlaceMode                   = guid(0xe2724bb8, 0xe676, 0x4806, [8]byte{0xb4, 0xb2, 0xa8, 0xd6, 0xef, 0xb4, 0x4c, 0xcd})
	mfMTAvgBitrate                      = guid(0x20332624, 0xfb0d, 0x4d9e, [8]byte{0xbd, 0x0d, 0xcb, 0xf6, 0x78, 0x6c, 0x10, 0x2e})
	mfSinkWriterDisableThrottling       = guid(0x08b845d8, 0x2b74, 0x4afe, [8]byte{0x9d, 0x53, 0xbe, 0x16, 0xd2, 0xd5, 0xae, 0x4f})
	mfReadwriteEnableHardwareTransforms = guid(0xa634a91c, 0x822b, 0x41b9, [8]byte{0xa4, 0x94, 0x4d, 0xe4, 0x64, 0x36, 0x12, 0xb0})
)

func guid(data1 uint32, data2, data3 uint16, data4 [8]byte) windows.GUID {
	return windows.GUID{Data1: data1, Data2: data2, Data3: data3, Data4: data4}
}

func initializeCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	if uint32(hr) == rpcEChangedMode {
		return false, nil
	}
	return true, checkHRESULT("CoInitializeEx", hr)
}

func uninitializeCOM() { procCoUninitialize.Call() }

func mfStartup() error {
	hr, _, _ := procMFStartup.Call(mfVersion, 0)
	return checkHRESULT("MFStartup", hr)
}

func mfShutdown() error {
	hr, _, _ := procMFShutdown.Call()
	return checkHRESULT("MFShutdown", hr)
}

func mfCreateSinkWriter(path *uint16, attrs uintptr, result *uintptr) uintptr {
	hr, _, _ := procMFCreateSinkWriterFromURL.Call(uintptr(unsafe.Pointer(path)), 0, attrs, uintptr(unsafe.Pointer(result)))
	return hr
}

func newAttributes(size uint32) (uintptr, error) {
	var value uintptr
	hr, _, _ := procMFCreateAttributes.Call(uintptr(unsafe.Pointer(&value)), uintptr(size))
	if err := checkHRESULT("MFCreateAttributes", hr); err != nil {
		return 0, err
	}
	return value, nil
}

func newMediaType() (uintptr, error) {
	var value uintptr
	hr, _, _ := procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&value)))
	if err := checkHRESULT("MFCreateMediaType", hr); err != nil {
		return 0, err
	}
	return value, nil
}

func mfCreateMemoryBuffer(size uint32, result *uintptr) uintptr {
	hr, _, _ := procMFCreateMemoryBuffer.Call(uintptr(size), uintptr(unsafe.Pointer(result)))
	return hr
}

func mfCreateSample(result *uintptr) uintptr {
	hr, _, _ := procMFCreateSample.Call(uintptr(unsafe.Pointer(result)))
	return hr
}

func setUINT32(object uintptr, key windows.GUID, value uint32) error {
	return checkHRESULT("IMFAttributes.SetUINT32", comCall(object, 21, uintptr(unsafe.Pointer(&key)), uintptr(value)))
}

func setUINT64(object uintptr, key windows.GUID, value uint64) error {
	return checkHRESULT("IMFAttributes.SetUINT64", comCall(object, 22, uintptr(unsafe.Pointer(&key)), uintptr(value)))
}

func setGUID(object uintptr, key, value windows.GUID) error {
	return checkHRESULT("IMFAttributes.SetGUID", comCall(object, 24, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))))
}

func comCall(object uintptr, index uintptr, args ...uintptr) uintptr {
	vtable := *(*uintptr)(unsafe.Add(unsafe.Pointer(nil), object))
	method := *(*uintptr)(unsafe.Add(unsafe.Pointer(nil), vtable+index*unsafe.Sizeof(uintptr(0))))
	callArgs := make([]uintptr, 1, len(args)+1)
	callArgs[0] = object
	callArgs = append(callArgs, args...)
	result, _, _ := syscall.SyscallN(method, callArgs...)
	return result
}

func releaseCOM(object uintptr) {
	if object != 0 {
		_ = comCall(object, 2)
	}
}

func checkHRESULT(name string, hr uintptr) error {
	if int32(uint32(hr)) < 0 {
		return fmt.Errorf("%s failed: HRESULT 0x%08x", name, uint32(hr))
	}
	return nil
}

func packUINT32(high, low int) uint64 {
	return uint64(uint32(high))<<32 | uint64(uint32(low)) //nolint:gosec // dimensions and FPS are validated
}
