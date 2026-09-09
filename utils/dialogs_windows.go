//go:build windows

package utils

import (
	"reflect"
	"runtime"
	"syscall"
)

// CredUI prompt flags for a password-only generic credential dialog:
// non-domain credential, username field pre-filled and locked, no
// certificate picker, and nothing is ever persisted.
const (
	credUIFlagsGenericCredentials  = 0x40000
	credUIFlagsKeepUsername        = 0x100000
	credUIFlagsPasswordOnlyOK      = 0x00200
	credUIFlagsExcludeCertificates = 0x00008
	credUIFlagsDoNotPersist        = 0x00002
)

const (
	credUIMaxUsernameLength = 513
	credUIMaxPasswordLength = 256
)

var (
	modcredui                       = syscall.NewLazyDLL("credui.dll")
	procCredUIPromptForCredentialsW = modcredui.NewProc("CredUIPromptForCredentialsW")
)

// credUIInfo mirrors CREDUI_INFOW. cbSize is set via reflection so the layout
// stays correct on all architectures.
type credUIInfo struct {
	cbSize      uint32
	hwndParent  uintptr
	messageText *uint16
	captionText *uint16
	banner      uintptr
}

// uptr converts a Go pointer to uintptr for Proc.Call without importing
// unsafe; the targets stay reachable via stack locals + runtime.KeepAlive.
func uptr(p interface{}) uintptr {
	return reflect.ValueOf(p).Pointer()
}

// PromptPassphrase shows the system credential dialog (credui.dll, with
// password masking) and returns the entered password. username pre-fills the
// locked username field so initial focus lands in the password box. It
// returns ok=false when the user cancels or the dialog fails.
func PromptPassphrase(caption, message, username string) (password string, ok bool) {
	// We run as a background tray process, so Windows may show the dialog
	// without giving it keyboard focus. Pin this goroutine to its OS thread
	// so the watcher can identify the dialog created by the blocking call
	// below, then force it to the foreground (mirrors RequestConfirm).
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	capPtr := syscall.StringToUTF16Ptr(caption)
	msgPtr := syscall.StringToUTF16Ptr(message)
	targetPtr := syscall.StringToUTF16Ptr("WinCryptSSHAgent")
	var userBuf [credUIMaxUsernameLength]uint16
	if u16, err := syscall.UTF16FromString(username); err == nil {
		if len(u16) > len(userBuf) {
			u16 = u16[:len(userBuf)]
			u16[len(u16)-1] = 0
		}
		copy(userBuf[:], u16)
	}
	var passBuf [credUIMaxPasswordLength]uint16
	var save uint32
	info := credUIInfo{
		messageText: msgPtr,
		captionText: capPtr,
	}
	info.cbSize = uint32(reflect.TypeOf(info).Size())
	raiseDialogWhenShown(currentThreadId())
	ret, _, _ := procCredUIPromptForCredentialsW.Call(
		uptr(&info),
		uptr(targetPtr),
		0, // pContext (reserved)
		0, // dwAuthError
		uptr(&userBuf[0]),
		uintptr(len(userBuf)),
		uptr(&passBuf[0]),
		uintptr(len(passBuf)),
		uptr(&save),
		uintptr(credUIFlagsGenericCredentials|credUIFlagsKeepUsername|credUIFlagsPasswordOnlyOK|credUIFlagsExcludeCertificates|credUIFlagsDoNotPersist),
	)
	runtime.KeepAlive(capPtr)
	runtime.KeepAlive(msgPtr)
	runtime.KeepAlive(targetPtr)
	if ret != 0 {
		return "", false
	}
	password = syscall.UTF16ToString(passBuf[:])
	for i := range passBuf {
		passBuf[i] = 0
	}
	return password, true
}

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
// the layout stays correct on all architectures.
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

const createNoWindow = 0x08000000

// askpassSysProcAttr hides the helper's console window.
func askpassSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
