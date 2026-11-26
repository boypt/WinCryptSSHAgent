package sshagent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/buptczq/WinCryptSSHAgent/capi"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows/registry"
)

const (
	savedKeyFilesValue = "SavedKeyFiles"
	savedCAPIKeysValue = "SavedCAPIKeyFingerprints"
)

var savedSourcesMu sync.Mutex
var savedKeyOperationMu sync.Mutex
var savedKeyGeneration uint64

func SavedKeyGeneration() uint64 {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	return savedKeyGeneration
}

func readSavedSources(valueName string) []string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\WinCryptSSHAgent`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	values, _, err := k.GetStringsValue(valueName)
	if err != nil {
		return nil
	}
	return values
}

func addSavedSource(valueName, value string) error {
	savedSourcesMu.Lock()
	defer savedSourcesMu.Unlock()
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\WinCryptSSHAgent`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	values, _, err := k.GetStringsValue(valueName)
	if err != nil {
		values = nil
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return nil
		}
	}
	values = append(values, value)
	return k.SetStringsValue(valueName, values)
}

func PersistKeyFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return addSavedSource(savedKeyFilesValue, filepath.Clean(abs))
}

// ImportKeyFile adds a user-selected key and remembers its path as one
// operation. A generation captured before the file picker prevents an import
// that was already in progress from undoing a later clear action.
func ImportKeyFile(ag agent.Agent, path, source string, generation uint64) error {
	key, err := ParseKeyFile(path, generation)
	if err != nil {
		return err
	}
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	if generation != savedKeyGeneration {
		return fmt.Errorf("key import cancelled because saved keys were cleared")
	}
	if err := addKeyWithSource(ag, key, source); err != nil {
		return err
	}
	return PersistKeyFile(path)
}

func certificateFingerprint(cert *capi.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func saveCAPICertificate(cert *capi.Certificate) error {
	return addSavedSource(savedCAPIKeysValue, certificateFingerprint(cert))
}

// ClearSavedKeySelections deletes only the remembered source references.
// Confirm mode shares this registry key and is left intact.
func clearSavedKeySelectionsLocked() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\WinCryptSSHAgent`, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer k.Close()
	for _, name := range []string{savedKeyFilesValue, savedCAPIKeysValue} {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
	}
	return nil
}

// RestoreSavedCertificates reloads only certificate fingerprints previously
// chosen from the Windows certificate picker.
func (s *CAPIAgent) RestoreSavedCertificates(generation uint64) {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	if generation != savedKeyGeneration {
		return
	}
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	wanted := make(map[string]bool)
	for _, fingerprint := range readSavedSources(savedCAPIKeysValue) {
		wanted[strings.ToUpper(fingerprint)] = true
	}
	if len(wanted) == 0 {
		return
	}
	certs, err := capi.LoadUserCerts()
	if err != nil {
		return
	}
	for _, cert := range certs {
		if !wanted[certificateFingerprint(cert)] {
			_ = cert.Free()
			continue
		}
		_ = s.addCertificate(cert, false)
	}
}
