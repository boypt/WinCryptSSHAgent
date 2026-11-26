package sshagent

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type WrappedAgent struct {
	agents []agent.Agent
}

// ImportCertificate opens the CAPI certificate picker when this wrapped agent
// includes the Windows certificate-store backend.
func (a *WrappedAgent) ImportCertificate() error {
	for _, backend := range a.agents {
		if capi, ok := backend.(*CAPIAgent); ok {
			return capi.ImportCertificate()
		}
	}
	return fmt.Errorf("Windows certificate-store keys are disabled")
}

// ClearSavedKeys removes loaded keys from both backends and forgets the
// references that would otherwise restore manually selected keys at startup.
func (a *WrappedAgent) ClearSavedKeys() error {
	savedKeyOperationMu.Lock()
	defer savedKeyOperationMu.Unlock()
	savedKeyGeneration++
	var firstErr error
	for _, backend := range a.agents {
		var err error
		switch keyAgent := backend.(type) {
		case *KeyRingAgent:
			err = keyAgent.RemoveAll()
		case *CAPIAgent:
			err = keyAgent.clearLoadedKeys()
		}
		if firstErr == nil && err != nil {
			firstErr = err
		}
	}
	clearSessionPassphrases()
	if err := clearSavedKeySelectionsLocked(); firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func NewWrappedAgent(defaultAgent agent.Agent, others []agent.Agent) *WrappedAgent {
	agents := []agent.Agent{defaultAgent}
	agents = append(agents, others...)

	return &WrappedAgent{
		agents: agents,
	}
}

func (a *WrappedAgent) List() ([]*agent.Key, error) {
	allKeys := make([]*agent.Key, 0)

	for _, agent := range a.agents {
		keys, err := agent.List()
		if err != nil {
			return nil, err
		}

		allKeys = append(allKeys, keys...)
	}

	return allKeys, nil
}

func (a *WrappedAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

func (a *WrappedAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.SignWithSource(key, data, flags, "")
}

func (a *WrappedAgent) SignWithSource(key ssh.PublicKey, data []byte, flags agent.SignatureFlags, source string) (*ssh.Signature, error) {
	var firstError error

	for _, agent_ := range a.agents {
		var sign *ssh.Signature
		var err error
		if srcAgent, ok := agent_.(SourceSigner); ok {
			sign, err = srcAgent.SignWithSource(key, data, flags, source)
		} else if extendAgent, ok := agent_.(agent.ExtendedAgent); ok {
			sign, err = extendAgent.SignWithFlags(key, data, flags)
		} else {
			sign, err = agent_.Sign(key, data)
		}

		if err == nil {
			return sign, nil
		}

		if firstError == nil {
			firstError = err
		}
	}

	return nil, firstError
}

func (a *WrappedAgent) Add(key agent.AddedKey) error {
	return a.agents[0].Add(key)
}

func (a *WrappedAgent) AddWithSource(key agent.AddedKey, source string) error {
	if notifier, ok := a.agents[0].(SourceNotifier); ok {
		return notifier.AddWithSource(key, source)
	}
	return a.agents[0].Add(key)
}

func (a *WrappedAgent) Remove(key ssh.PublicKey) error {
	return a.agents[0].Remove(key)
}

func (a *WrappedAgent) RemoveWithSource(key ssh.PublicKey, source string) error {
	if notifier, ok := a.agents[0].(SourceNotifier); ok {
		return notifier.RemoveWithSource(key, source)
	}
	return a.agents[0].Remove(key)
}

func (a *WrappedAgent) RemoveAll() error {
	return a.agents[0].RemoveAll()
}

func (a *WrappedAgent) RemoveAllWithSource(source string) error {
	if notifier, ok := a.agents[0].(SourceNotifier); ok {
		return notifier.RemoveAllWithSource(source)
	}
	return a.agents[0].RemoveAll()
}

func (a *WrappedAgent) Lock(passphrase []byte) error {
	return a.agents[0].Lock(passphrase)
}

func (a *WrappedAgent) Unlock(passphrase []byte) error {
	return a.agents[0].Unlock(passphrase)
}

func (a *WrappedAgent) Signers() ([]ssh.Signer, error) {
	return a.agents[0].Signers()
}

func (a *WrappedAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}
