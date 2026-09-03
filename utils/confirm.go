package utils

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// ConfirmRequired controls whether signing requests require user confirmation.
// When false (default), signing proceeds automatically (Auto Confirm).
// When true, a blocking dialog is shown before each signing operation (Manual Confirm).
var ConfirmRequired bool

// confirmRegPath is the HKCU registry key holding the persisted confirm state.
const confirmRegPath = `Software\WinCryptSSHAgent`

// confirmRegName is the DWORD value (0 = auto, 1 = manual) storing the state.
const confirmRegName = "ConfirmRequired"

// LoadConfirmFromRegistry reads the persisted confirm state. It returns the
// value and whether a value was actually present in the registry.
func LoadConfirmFromRegistry() (bool, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, confirmRegPath, registry.QUERY_VALUE)
	if err != nil {
		return false, false
	}
	defer key.Close()
	val, _, err := key.GetIntegerValue(confirmRegName)
	if err != nil {
		return false, false
	}
	return val != 0, true
}

// SaveConfirmToRegistry persists the confirm state so it survives restarts.
func SaveConfirmToRegistry(manual bool) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, confirmRegPath, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	val := uint64(0)
	if manual {
		val = 1
	}
	_ = key.SetDWordValue(confirmRegName, uint32(val))
}

// RequestConfirm shows a blocking Yes/No dialog and returns true if the user
// clicked Yes.
func RequestConfirm(title, message string) bool {
	style := uintptr(MB_YESNO | MB_ICONQUESTION | MB_SYSTEMMODAL | MB_TOPMOST | MB_SETFOREGROUND)
	ret := MessageBox(title, message, style)
	return ret == IDYES
}

// ConfirmSign builds a signing-confirmation dialog for the given key info and
// source, then blocks until the user authorises or denies.
func ConfirmSign(comment, fingerprint, source string) bool {
	msg := fmt.Sprintf("SSH signing request:\nKey: %s [%s]", comment, fingerprint)
	if source != "" {
		msg += fmt.Sprintf("\nSource: %s", source)
	}
	msg += "\n\nDo you want to authorise this signing?"
	return RequestConfirm("SSH Signing Request", msg)
}
