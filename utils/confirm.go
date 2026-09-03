package utils

import "fmt"

// ConfirmRequired controls whether signing requests require user confirmation.
// When false (default), signing proceeds automatically (backward-compatible).
// When true, a blocking dialog is shown before each signing operation.
var ConfirmRequired bool

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
