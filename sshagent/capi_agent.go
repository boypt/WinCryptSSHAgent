package sshagent

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/buptczq/WinCryptSSHAgent/capi"
	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type sshKey struct {
	cert    *capi.Certificate
	signer  ssh.Signer
	comment string
}

type CAPIAgent struct {
	mu              sync.Mutex
	loadMu          sync.Mutex
	keys            []*sshKey
	autoLoad        bool
	loaded          bool
	touchNotify     bool
	clearGeneration uint64
}

func (s *CAPIAgent) SetAutoLoad(enabled bool) { s.autoLoad = enabled }

func (s *CAPIAgent) SetTouchNotification(enabled bool) { s.touchNotify = enabled }

func (s *CAPIAgent) close() (err error) {
	for _, key := range s.keys {
		if freeErr := key.cert.Free(); err == nil && freeErr != nil {
			err = freeErr
		}
	}
	s.keys = nil
	return
}

func (s *CAPIAgent) Close() (err error) {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.close()
}

// ClearLoadedKeys clears this process's loaded CAPI keys and suppresses a
// subsequent automatic reload until the next process starts.
func (s *CAPIAgent) ClearLoadedKeys() error {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	savedKeyGeneration++
	return s.clearLoadedKeys()
}

func (s *CAPIAgent) clearLoadedKeys() error {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoLoad = false
	s.loaded = true
	s.clearGeneration++
	return s.close()
}

func certDisplayName(cert *capi.Certificate) string {
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	if cert.Subject.String() != "" {
		return cert.Subject.String()
	}
	if cert.SerialNumber != nil {
		return fmt.Sprintf("Serial-%s", cert.SerialNumber.String())
	}
	return "Unknown Certificate"
}

func (s *CAPIAgent) ensureLoaded(refresh bool) error {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if !s.autoLoad {
		return nil
	}
	s.mu.Lock()
	loaded := s.loaded
	s.mu.Unlock()
	if loaded && !refresh {
		return nil
	}
	certs, err := capi.LoadUserCerts()
	if err != nil {
		return err
	}
	candidate := &CAPIAgent{}
	for _, cert := range certs {
		_ = candidate.addCertificate(cert, false)
	}
	s.mu.Lock()
	oldKeys := s.keys
	s.keys = candidate.keys
	candidate.keys = nil
	s.loaded = true
	s.mu.Unlock()
	old := &CAPIAgent{keys: oldKeys}
	if err := old.close(); err != nil {
		log.Printf("capi: could not release refreshed certificate context: %v", err)
	}
	return nil
}

func (s *CAPIAgent) List() (keys []*agent.Key, err error) {
	if err = s.ensureLoaded(true); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]*agent.Key, 0, len(s.keys))
	for _, k := range s.keys {
		pub := k.signer.PublicKey()
		ids = append(ids, &agent.Key{
			Format:  pub.Type(),
			Blob:    pub.Marshal(),
			Comment: k.comment})
	}
	return ids, nil
}

// AddCertificate adds one explicitly selected Windows certificate key.
// Ownership of cert transfers to the agent, including when validation fails.
func (s *CAPIAgent) AddCertificate(cert *capi.Certificate) error {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	return s.addCertificate(cert, true)
}

func (s *CAPIAgent) addCertificate(cert *capi.Certificate, persist bool) error {
	if cert == nil {
		return nil
	}
	keep := false
	defer func() {
		if !keep {
			_ = cert.Free()
		}
	}()
	if !FilterCertificateEKU(cert) {
		return fmt.Errorf("certificate is not valid for SSH authentication")
	}
	pub, err := ssh.NewPublicKey(cert.PublicKey)
	if err != nil {
		return err
	}
	var signer ssh.Signer
	switch pub.Type() {
	case ssh.KeyAlgoRSA:
		signer = &rsaSigner{pub: pub, cert: cert}
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		signer = &ecdsaSigner{pub: pub, cert: cert}
	default:
		return fmt.Errorf("unsupported Windows certificate key type: %s", pub.Type())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.keys {
		if bytes.Equal(existing.signer.PublicKey().Marshal(), pub.Marshal()) {
			if persist {
				if err := saveCAPICertificate(cert); err != nil {
					log.Printf("capi: could not save selected certificate reference: %v", err)
					return err
				}
				return nil
			}
			return fmt.Errorf("certificate key is already loaded")
		}
	}
	key := &sshKey{cert: cert, signer: signer, comment: certDisplayName(cert)}
	s.keys = append(s.keys, key)
	if keyWithCert, err := loadSSHCertificate(key); err == nil {
		s.keys = append(s.keys, keyWithCert)
	}
	keep = true
	if persist {
		if err := saveCAPICertificate(cert); err != nil {
			log.Printf("capi: could not save selected certificate reference: %v", err)
		}
	}
	return nil
}

// ImportCertificate opens the Windows Current User\\Personal picker and adds
// only the certificate the user selects.
func (s *CAPIAgent) ImportCertificate() error {
	generation := SavedKeyGeneration()
	cert, err := capi.SelectUserCertificate()
	if err != nil || cert == nil {
		return err
	}
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if generation != savedKeyGeneration {
		_ = cert.Free()
		return fmt.Errorf("certificate import cancelled because saved keys were cleared")
	}
	return s.addCertificate(cert, true)
}

func (s *CAPIAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return s.SignWithFlags(key, data, 0)
}

func (s *CAPIAgent) signed(comment, source string) {
	// Manual Confirm 已弹窗告知用户，跳过通知 / the confirm dialog already
	// informed the user, so skip the "Authenticated" toast.
	if utils.ConfirmRequired || utils.AuthNotificationDisabled {
		return
	}
	title := "Authenticated"
	msg := fmt.Sprintf("Key: <%s>", comment)
	if source != "" {
		msg = fmt.Sprintf("Key: <%s>\nSource: %s", comment, source)
	}
	utils.NotifyAuth(title, msg)
}

func (s *CAPIAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return s.SignWithSource(key, data, flags, "")
}

func (s *CAPIAgent) SignWithSource(key ssh.PublicKey, data []byte, flags agent.SignatureFlags, source string) (*ssh.Signature, error) {
	if err := s.ensureLoaded(false); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if os.Getenv("WCSA_CHECKSVR") == "1" {
		if ok, err := utils.CheckSCardSvrStatus(); err == nil && !ok {
			if utils.MessageBox("Warning:", "Smart Card Service is stopped! Do you want to restart it?", utils.MB_OKCANCEL) == utils.IDOK {
				utils.StartSCardSvr()
			}
		}
	}

	wanted := key.Marshal()
	var matched *sshKey
	for _, k := range s.keys {
		if bytes.Equal(k.signer.PublicKey().Marshal(), wanted) {
			matched = k
			break
		}
	}
	if matched == nil {
		return nil, errors.New("not found")
	}
	// 签名确认 / signing confirmation gate
	if utils.ConfirmRequired {
		comment := matched.comment
		fingerprint := ssh.FingerprintSHA256(matched.signer.PublicKey())
		clearGeneration := s.clearGeneration
		s.mu.Unlock()
		ok := utils.ConfirmSign(comment, fingerprint, source)
		s.mu.Lock()
		if !ok {
			return nil, fmt.Errorf("signing denied by user")
		}
		if clearGeneration != s.clearGeneration {
			return nil, fmt.Errorf("key was cleared while signing confirmation was open")
		}
		current := false
		for _, loadedKey := range s.keys {
			if bytes.Equal(loadedKey.signer.PublicKey().Marshal(), wanted) {
				matched = loadedKey
				current = true
				break
			}
		}
		if !current {
			return nil, fmt.Errorf("key was removed while signing confirmation was open")
		}
	}

	if flags == 0 {
		stopTouchPrompt := func() {}
		if s.touchNotify {
			stopTouchPrompt = notifyTouchPrompt(matched.comment)
		}
		sign, err := matched.signer.Sign(rand.Reader, data)
		stopTouchPrompt()
		if err == nil {
			s.signed(matched.comment, source)
		}
		return sign, err
	}

	algorithmSigner, ok := matched.signer.(ssh.AlgorithmSigner)
	if !ok {
		return nil, fmt.Errorf("agent: signature does not support non-default signature algorithm: %T", matched.signer)
	}
	var algorithm string
	switch flags {
	case agent.SignatureFlagRsaSha256:
		algorithm = ssh.SigAlgoRSASHA2256
	case agent.SignatureFlagRsaSha512:
		algorithm = ssh.SigAlgoRSASHA2512
	default:
		return nil, fmt.Errorf("agent: unsupported signature flags: %d", flags)
	}
	stopTouchPrompt := func() {}
	if s.touchNotify {
		stopTouchPrompt = notifyTouchPrompt(matched.comment)
	}
	sign, err := algorithmSigner.SignWithAlgorithm(rand.Reader, data, algorithm)
	stopTouchPrompt()
	if err == nil {
		s.signed(matched.comment, source)
	}
	return sign, err
}

// notifyTouchPrompt waits briefly before notifying so fast signatures, including
// those satisfied by a cached YubiKey touch, do not prompt. Windows CNG does
// not expose whether a provider is specifically waiting for physical touch.
func notifyTouchPrompt(comment string) func() {
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(1500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			utils.Notify("Touch Security Key", fmt.Sprintf("If your security key is waiting, touch it to sign with <%s>.", comment))
		case <-done:
		}
	}()
	return func() { close(done) }
}

func (*CAPIAgent) Add(key agent.AddedKey) error {
	return fmt.Errorf("implement me")
}

func (*CAPIAgent) Remove(key ssh.PublicKey) error {
	return fmt.Errorf("implement me")
}

func (s *CAPIAgent) RemoveAll() error {
	return s.ClearLoadedKeys()
}

func (*CAPIAgent) Lock(passphrase []byte) error {
	return fmt.Errorf("implement me")
}

func (*CAPIAgent) Unlock(passphrase []byte) error {
	return fmt.Errorf("implement me")
}

func (*CAPIAgent) Signers() ([]ssh.Signer, error) {
	return nil, fmt.Errorf("implement me")
}

func (s *CAPIAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}
