package utils

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// maxAskPassOutput caps collected askpass stdout (silently truncated).
const maxAskPassOutput = 1023

// askPassTimeout bounds one helper invocation. Upstream OpenSSH waits forever
// (waitpid with no timeout), but a GUI agent must never hang startup or the
// tray event loop on a helper that never exits; on timeout the helper is
// killed and treated as cancel.
const askPassTimeout = 60 * time.Second

// AskPassOutcome describes how a WCSA_ASKPASS helper invocation ended.
type AskPassOutcome int

const (
	// AskPassUnavailable: the helper never ran (not set / not found /
	// DevNull failure / timeout kill) — the caller should try the next
	// passphrase method.
	AskPassUnavailable AskPassOutcome = iota
	// AskPassSucceeded: exit 0 with non-empty output — use the password.
	AskPassSucceeded
	// AskPassCancelled: the helper ran but gave no password (non-zero exit,
	// or exit 0 with empty output) — the user cancelled in the helper, so
	// the caller should skip the key without further prompting.
	AskPassCancelled
)

// RunAskPass runs the WCSA_ASKPASS helper for prompt and returns its password
// output. Simplified gating: if WCSA_ASKPASS is set, the helper is used — no
// DISPLAY/WAYLAND/SSH_ASKPASS_REQUIRE checks. The helper is invoked as
// [prog, prompt] with the full environment inherited and stdin on DevNull;
// output is truncated at 1023 bytes and at the first \r or \n, and the
// invocation is killed after 60s. Password content itself is never logged,
// only lengths and outcomes.
func RunAskPass(prompt string) (string, AskPassOutcome) {
	prog := os.Getenv("WCSA_ASKPASS")
	if prog == "" {
		log.Printf("askpass: WCSA_ASKPASS not set, skip (prompt=%q)", prompt)
		return "", AskPassUnavailable
	}
	path, err := exec.LookPath(prog)
	if err != nil {
		log.Printf("askpass: LookPath(%q) failed: %v", prog, err)
		return "", AskPassUnavailable
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		log.Printf("askpass: open DevNull failed: %v", err)
		return "", AskPassUnavailable
	}
	defer devNull.Close()
	var attr *syscall.SysProcAttr = askpassSysProcAttr()
	ctx, cancel := context.WithTimeout(context.Background(), askPassTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, prompt)
	cmd.Env = os.Environ()
	cmd.Stdin = devNull
	cmd.SysProcAttr = attr
	log.Printf("askpass: running %q (resolved %q) with 1 prompt argv, stdin=DevNull", prog, path)
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("askpass: %q timed out after %v, killed (next method)", path, askPassTimeout)
		return "", AskPassUnavailable
	}
	if err != nil {
		log.Printf("askpass: %q failed: %v (treated as cancel)", path, err)
		return "", AskPassCancelled
	}
	rawLen := len(out)
	if len(out) > maxAskPassOutput {
		out = out[:maxAskPassOutput]
	}
	cut := -1
	if i := strings.IndexAny(string(out), "\r\n"); i >= 0 {
		out = out[:i]
		cut = i
	}
	if len(out) == 0 {
		log.Printf("askpass: %q exit 0 with empty output (treated as cancel)", path)
		return "", AskPassCancelled
	}
	log.Printf("askpass: %q exit 0, raw=%dB truncated=%dB newlineCut=%d adoptedLen=%d", path, rawLen, len(out), cut, len(out))
	return string(out), AskPassSucceeded
}
