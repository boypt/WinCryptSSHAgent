//go:build !windows

package utils

// OpenFileDialog is a non-Windows stub: no file dialog is available,
// so file selection always reports cancellation.
func OpenFileDialog(title string) (string, bool) {
	return "", false
}
