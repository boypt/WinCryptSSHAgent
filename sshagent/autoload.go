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

// Manual private-key import helpers.
var sessionPassphrasesMu sync.Mutex

// ErrDecryptFailed marks a key that had a passphrase but still failed to
// decrypt. Callers use errors.Is to tell it apart from cancellations and
// read/parse errors.
var ErrDecryptFailed = errors.New("decrypt failed")

var sessionPassphrases []string // process-lifetime cache of successfully entered passphrases (memory only, never persisted or logged)

func clearSessionPassphrases() {
	sessionPassphrasesMu.Lock()
	sessionPassphrases = nil
	sessionPassphrasesMu.Unlock()
}

func ParseKeyFile(path string, generation uint64) (agent.AddedKey, error) {
	if SavedKeyGeneration() != generation {
		return agent.AddedKey{}, fmt.Errorf("key import cancelled because saved keys were cleared")
	}
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
		sessionPassphrasesMu.Lock()
		cachedPassphrases := append([]string(nil), sessionPassphrases...)
		sessionPassphrasesMu.Unlock()
		for _, pp := range cachedPassphrases {
			if k, err := ssh.ParseRawPrivateKeyWithPassphrase(raw, []byte(pp)); err == nil {
				return agent.AddedKey{PrivateKey: k, Comment: base}, nil
			}
		}
		keyName := strings.TrimSuffix(base, filepath.Ext(base))
		if keyName == "" {
			keyName = base
		}
		promptMsg := fmt.Sprintf("Enter passphrase for %s:", keyName)
		if SavedKeyGeneration() != generation {
			return agent.AddedKey{}, fmt.Errorf("key import cancelled because saved keys were cleared")
		}
		pp, ok := utils.PromptPassphrase("Import Key", promptMsg, keyName)
		if !ok {
			return agent.AddedKey{}, err
		}
		if k, perr := ssh.ParseRawPrivateKeyWithPassphrase(raw, []byte(pp)); perr == nil {
			cachePassphraseIfCurrent(generation, pp)
			return agent.AddedKey{PrivateKey: k, Comment: base}, nil
		} else {
			return agent.AddedKey{}, fmt.Errorf("%w for <%s>: %v", ErrDecryptFailed, base, perr)
		}
	}
	return agent.AddedKey{PrivateKey: key, Comment: filepath.Base(path)}, nil
}

// DefaultKeyFiles returns private keys from ~/.ssh without public keys,
// certificate files, or directories.
func DefaultKeyFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".ssh", "id_*"))
	files := make([]string, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".pub") || strings.HasSuffix(base, ".cert") {
			continue
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		files = append(files, path)
	}
	return files
}

// AutoLoadKeys restores manually selected file paths and, when enabled,
// scans the standard ~/.ssh key names at startup.
func AutoLoadKeys(keyring *KeyRingAgent, scanDefaults bool, generation uint64) {
	seen := make(map[string]bool)
	add := func(path, source string) bool {
		pathKey := strings.ToLower(filepath.Clean(path))
		if seen[pathKey] {
			return true
		}
		seen[pathKey] = true
		parsedKey, err := ParseKeyFile(path, generation)
		if err != nil {
			log.Printf("key import: skip <%s> source=%s: %v", filepath.Base(path), source, err)
			return SavedKeyGeneration() == generation
		}
		savedKeyOperationMu.Lock()
		defer savedKeyOperationMu.Unlock()
		if savedKeyGeneration != generation {
			return false
		}
		if err := addKeyWithSource(keyring, parsedKey, source); err != nil {
			log.Printf("key import: add <%s> source=%s failed: %v", parsedKey.Comment, source, err)
			return true
		}
		log.Printf("key import: added <%s> source=%s", parsedKey.Comment, source)
		return true
	}
	for _, path := range readSavedSources(savedKeyFilesValue) {
		if !add(path, "Saved Import") {
			return
		}
	}
	if scanDefaults {
		for _, path := range DefaultKeyFiles() {
			if !add(path, "AutoLoad") {
				return
			}
		}
	}
}

func addKeyWithSource(ag agent.Agent, key agent.AddedKey, source string) error {
	var err error
	if n, ok := ag.(SourceNotifier); ok {
		err = n.AddWithSource(key, source)
	} else {
		err = ag.Add(key)
	}
	return err
}

func cachePassphraseIfCurrent(generation uint64, passphrase string) {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	if savedKeyGeneration != generation {
		return
	}
	sessionPassphrasesMu.Lock()
	sessionPassphrases = append(sessionPassphrases, passphrase)
	sessionPassphrasesMu.Unlock()
}
