//go:build !windows

package utils

// PromptPassphrase is a non-Windows stub: no credential UI is available,
// so passphrase prompting always reports cancellation.
func PromptPassphrase(caption, message string) (string, bool) {
	return "", false
}
