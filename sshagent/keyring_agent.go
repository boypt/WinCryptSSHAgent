package sshagent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type KeyRingAgent struct {
	ag agent.ExtendedAgent
}

func NewKeyRingAgent() *KeyRingAgent {
	return &KeyRingAgent{
		ag: agent.NewKeyring().(agent.ExtendedAgent),
	}
}

func (s *KeyRingAgent) List() ([]*agent.Key, error) {
	return s.ag.List()
}

func (s *KeyRingAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return s.SignWithFlags(key, data, 0)
}

func (s *KeyRingAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return s.SignWithSource(key, data, flags, "")
}

func (s *KeyRingAgent) SignWithSource(key ssh.PublicKey, data []byte, flags agent.SignatureFlags, source string) (*ssh.Signature, error) {
	comment := s.findKeyComment(key)

	// 签名确认 / signing confirmation gate
	if utils.ConfirmRequired {
		fp := ssh.FingerprintSHA256(key)
		if !utils.ConfirmSign(comment, fp, source) {
			return nil, fmt.Errorf("signing denied by user")
		}
	}

	sig, err := s.ag.SignWithFlags(key, data, flags)
	if err == nil {
		s.signed(comment, source)
	}
	return sig, err
}

func (s *KeyRingAgent) findKeyComment(pubkey ssh.PublicKey) string {
	wanted := pubkey.Marshal()
	keys, err := s.List()
	if err != nil {
		goto fallback
	}
	for _, k := range keys {
		if bytes.Equal(k.Marshal(), wanted) {
			return k.Comment
		}
	}
fallback:
	return base64.StdEncoding.EncodeToString(wanted)
}

func (s *KeyRingAgent) AddWithSource(key agent.AddedKey, source string) error {
    err := s.ag.Add(key)
    if err == nil {
        msg := fmt.Sprintf("Key <%s> has been added to keyring", key.Comment)
        if source != "" {
            msg = fmt.Sprintf("%s\nSource: %s", msg, source)
        }
        utils.NotifyAdd("Key Added", msg)
    }
    return err
}

// Add implements agent.Agent.Add and forwards to AddWithSource with empty source.
func (s *KeyRingAgent) Add(key agent.AddedKey) error {
    return s.AddWithSource(key, "")
}

func (s *KeyRingAgent) RemoveWithSource(key ssh.PublicKey, source string) error {
    comment := s.findKeyComment(key)
    err := s.ag.Remove(key)
    if err == nil {
        msg := fmt.Sprintf("Key <%s> has been removed from keyring", comment)
        if source != "" {
            msg = fmt.Sprintf("%s\nSource: %s", msg, source)
        }
        utils.NotifyRemove("Key Removed", msg)
    }
    return err
}

func (s *KeyRingAgent) RemoveAllWithSource(source string) error {
    err := s.ag.RemoveAll()
    if err == nil {
        msg := "All Keys have been removed from keyring"
        if source != "" {
            msg = fmt.Sprintf("%s\nSource: %s", msg, source)
        }
        utils.NotifyRemove("Key Removed", msg)
    }
    return err
}

func (s *KeyRingAgent) Remove(key ssh.PublicKey) error {
	comment := s.findKeyComment(key)
	err := s.ag.Remove(key)
	if err == nil {
		defer utils.NotifyRemove(
			"Key Removed",
			"Key <"+comment+"> has been removed from keyring",
		)
	}
	return err
}

func (s *KeyRingAgent) RemoveAll() error {
	err := s.ag.RemoveAll()
	if err == nil {
		defer utils.NotifyRemove(
			"Key Removed",
			"All Keys have been removed from keyring",
		)
	}
	return err
}

func (s *KeyRingAgent) Lock(passphrase []byte) error {
	return s.ag.Lock(passphrase)
}

func (s *KeyRingAgent) Unlock(passphrase []byte) error {
	return s.ag.Unlock(passphrase)
}

func (s *KeyRingAgent) Signers() ([]ssh.Signer, error) {
	return s.ag.Signers()
}

func (s *KeyRingAgent) signed(comment, source string) {
	// Manual Confirm 已弹窗告知用户，跳过通知 / the confirm dialog already
	// informed the user, so skip the "Authenticated" toast.
	if utils.ConfirmRequired {
		return
	}
	title := "Authenticated (Keyring)"
	msg := fmt.Sprintf("Key: <%s>", comment)
	if source != "" {
		msg = fmt.Sprintf("Key: <%s>\nSource: %s", comment, source)
	}
	utils.NotifyAuth(title, msg)
}

func (s *KeyRingAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}
