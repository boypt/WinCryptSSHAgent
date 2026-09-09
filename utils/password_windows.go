//go:build windows

package utils

import (
	"reflect"
	"runtime"
	"syscall"
)

// CredUI prompt flags for a password-only generic credential dialog:
// non-domain credential, username field hidden, no certificate picker,
// and nothing is ever persisted.
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
// stays correct on both 386 and amd64.
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
// password masking) and returns the entered password. It returns ok=false
// when the user cancels or the dialog fails.
func PromptPassphrase(caption, message string) (password string, ok bool) {
	capPtr := syscall.StringToUTF16Ptr(caption)
	msgPtr := syscall.StringToUTF16Ptr(message)
	targetPtr := syscall.StringToUTF16Ptr("WinCryptSSHAgent")
	var userBuf [credUIMaxUsernameLength]uint16
	var passBuf [credUIMaxPasswordLength]uint16
	var save uint32
	info := credUIInfo{
		messageText: msgPtr,
		captionText: capPtr,
	}
	info.cbSize = uint32(reflect.TypeOf(info).Size())
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
