package sshagent

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Key auto-load / manual import shared helper.
//
// WCSA_KEYS: extra private-key file paths separated by os.PathListSeparator
// (';' on Windows, ':' elsewhere); entries containing spaces are fine, only
// the separator splits. WCSA_KEY_PASSPHRASE: passphrase tried first for every
// encrypted key; kept only in memory and never logged.
// importMu serializes startup auto-import vs tray manual import and guards
// sessionPassphrases (keyring Add and Notify have their own mutexes).
var importMu sync.Mutex

// ErrDecryptFailed marks a key that had a passphrase but still failed to
// decrypt. Callers use errors.Is to tell it apart from cancellations and
// read/parse errors.
var ErrDecryptFailed = errors.New("decrypt failed")

var sessionPassphrases []string // process-lifetime cache of successfully entered passphrases (memory only, never persisted or logged)

func ParseKeyFile(path string) (agent.AddedKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return agent.AddedKey{}, err
	}
	key, err := ssh.ParseRawPrivateKey(raw)
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if !errors.As(err, &missing) {
			return agent.AddedKey{}, err
		}
		base := filepath.Base(path)
		candidates := sessionPassphrases
		if pp := os.Getenv("WCSA_KEY_PASSPHRASE"); pp != "" {
			candidates = append([]string{pp}, candidates...)
		}
		for _, pp := range candidates {
			if k, err := ssh.ParseRawPrivateKeyWithPassphrase(raw, []byte(pp)); err == nil {
				return agent.AddedKey{PrivateKey: k, Comment: base}, nil
			}
		}
		keyName := strings.TrimSuffix(base, filepath.Ext(base))
		if keyName == "" {
			keyName = base
		}
		promptMsg := fmt.Sprintf("Enter passphrase for %s:", keyName)
		switch pp, outcome := utils.RunAskPass(promptMsg); outcome {
		case utils.AskPassSucceeded:
			if k, err := ssh.ParseRawPrivateKeyWithPassphrase(raw, []byte(pp)); err == nil {
				sessionPassphrases = append(sessionPassphrases, pp)
				return agent.AddedKey{PrivateKey: k, Comment: base}, nil
			}
			// Wrong password: fall through to CredUI for another chance.
		case utils.AskPassCancelled:
			return agent.AddedKey{}, fmt.Errorf("askpass cancelled for <%s>", base)
		case utils.AskPassUnavailable:
			// Helper never ran; fall through to CredUI unchanged.
		}
		pp, ok := utils.PromptPassphrase("Import Key", promptMsg, keyName)
		if !ok {
			return agent.AddedKey{}, err
		}
		if k, perr := ssh.ParseRawPrivateKeyWithPassphrase(raw, []byte(pp)); perr == nil {
			sessionPassphrases = append(sessionPassphrases, pp)
			return agent.AddedKey{PrivateKey: k, Comment: base}, nil
		} else {
			return agent.AddedKey{}, fmt.Errorf("%w for <%s>: %v", ErrDecryptFailed, base, perr)
		}
	}
	return agent.AddedKey{PrivateKey: key, Comment: filepath.Base(path)}, nil
}

// DefaultKeyFiles returns ~/.ssh/id_* (excluding *.pub, *.cert — the latter
// also covers *cert.pub) plus WCSA_KEYS entries, de-duplicated.
func DefaultKeyFiles() []string {
	seen := make(map[string]bool)
	var files []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		files = append(files, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		matches, _ := filepath.Glob(filepath.Join(home, ".ssh", "id_*"))
		for _, m := range matches {
			base := filepath.Base(m)
			if strings.HasSuffix(base, ".pub") || strings.HasSuffix(base, ".cert") {
				continue
			}
			if st, err := os.Stat(m); err != nil || st.IsDir() {
				continue
			}
			add(m)
		}
	}
	for _, p := range strings.Split(os.Getenv("WCSA_KEYS"), string(os.PathListSeparator)) {
		if p = strings.TrimSpace(p); p != "" {
			add(p)
		}
	}
	return files
}

// AddKeyFile parses path with ParseKeyFile and adds it to ag, tagging Source.
// Decrypt failures show a warning dialog; other failures (cancelled, read or
// parse errors) are skipped with a toast, never blocking.
func AddKeyFile(ag agent.Agent, path, source string) error {
	importMu.Lock()
	defer importMu.Unlock()
	key, err := ParseKeyFile(path)
	if err != nil {
		log.Printf("autoload: skip <%s> source=%s: %v", filepath.Base(path), source, err)
		if errors.Is(err, ErrDecryptFailed) {
			utils.MessageBoxForeground("Import Key", fmt.Sprintf("Could not unlock key:\n%v", err), utils.MB_ICONWARNING)
		} else {
			utils.Notify("Import Key", fmt.Sprintf("Skipped <%s>: %v", filepath.Base(path), err))
		}
		return err
	}
	if n, ok := ag.(SourceNotifier); ok {
		err = n.AddWithSource(key, source)
	} else {
		err = ag.Add(key)
	}
	if err != nil {
		log.Printf("autoload: add <%s> source=%s failed: %v", key.Comment, source, err)
		return err
	}
	log.Printf("autoload: added <%s> source=%s", key.Comment, source)
	return nil
}

// AutoLoadKeys imports DefaultKeyFiles into keyring once at startup.
// Encrypted keys fall back to an interactive passphrase prompt; cancelled
// keys are skipped with a toast, decrypt failures show a warning dialog,
// and neither blocks other files.
func AutoLoadKeys(keyring *KeyRingAgent) {
	files := DefaultKeyFiles()
	log.Printf("autoload: start, %d file(s)", len(files))
	for _, f := range files {
		_ = AddKeyFile(keyring, f, "AutoLoad")
	}
	// WCSA_KEYS / WCSA_KEY_PASSPHRASE / WCSA_ASKPASS are honored only during
	// startup auto-load. Clear them afterwards so they never linger in the
	// process environment (which child processes inherit) and so manual tray
	// imports always use the built-in dialog. Only the names are logged,
	// never any values.
	for _, name := range []string{"WCSA_KEYS", "WCSA_KEY_PASSPHRASE", "WCSA_ASKPASS"} {
		if err := os.Unsetenv(name); err != nil {
			log.Printf("autoload: unset %s failed: %v", name, err)
		} else {
			log.Printf("autoload: cleared %s", name)
		}
	}
	log.Printf("autoload: done")
}
