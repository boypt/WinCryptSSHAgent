//go:build windows

package utils

import (
	"reflect"
	"runtime"
	"syscall"
)

// Open-file dialog flags: Explorer-style dialog, the selection must be an
// existing file on an existing path, and the process CWD is left alone.
const (
	ofnExplorer      = 0x80000
	ofnFileMustExist = 0x1000
	ofnPathMustExist = 0x800
	ofnNoChangeDir   = 0x8
)

// maxFileDialogPath is the caller-owned file buffer in UTF-16 code units
// (32768 covers long paths).
const maxFileDialogPath = 32768

var (
	modcomdlg32          = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW = modcomdlg32.NewProc("GetOpenFileNameW")
)

// openFileName mirrors OPENFILENAMEW. structSize is set via reflection so
// the layout stays correct on both 386 and amd64.
type openFileName struct {
	structSize    uint32
	hwndOwner     uintptr
	hInstance     uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
}

// OpenFileDialog shows the system open-file dialog with the given title and
// returns the selected path. Cancel and failure both return ("", false).
func OpenFileDialog(title string) (path string, ok bool) {
	titlePtr := syscall.StringToUTF16Ptr(title)
	var fileBuf [maxFileDialogPath]uint16
	ofn := openFileName{
		file:    &fileBuf[0],
		maxFile: uint32(len(fileBuf)),
		title:   titlePtr,
		flags:   ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}
	ofn.structSize = uint32(reflect.TypeOf(ofn).Size())
	ret, _, _ := procGetOpenFileNameW.Call(uptr(&ofn))
	runtime.KeepAlive(titlePtr)
	if ret == 0 {
		return "", false
	}
	path = syscall.UTF16ToString(fileBuf[:])
	if path == "" {
		return "", false
	}
	return path, true
}
