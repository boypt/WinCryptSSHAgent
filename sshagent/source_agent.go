package sshagent

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SourceSigner represents an agent that supports tagging signature requests with a source channel.
type SourceSigner interface {
	SignWithSource(key ssh.PublicKey, data []byte, flags agent.SignatureFlags, source string) (*ssh.Signature, error)
}

// SourceAgent wraps an agent.Agent to associate incoming requests with a specific source channel name.
type SourceAgent struct {
	agent  agent.Agent
	source string
}

func NewSourceAgent(ag agent.Agent, source string) agent.ExtendedAgent {
	return &SourceAgent{
		agent:  ag,
		source: source,
	}
}

func (s *SourceAgent) List() ([]*agent.Key, error) {
	return s.agent.List()
}

func (s *SourceAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return s.SignWithFlags(key, data, 0)
}

func (s *SourceAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	if srcAgent, ok := s.agent.(SourceSigner); ok {
		return srcAgent.SignWithSource(key, data, flags, s.source)
	}
	if extAgent, ok := s.agent.(agent.ExtendedAgent); ok {
		return extAgent.SignWithFlags(key, data, flags)
	}
	return s.agent.Sign(key, data)
}

func (s *SourceAgent) Add(key agent.AddedKey) error {
	return s.agent.Add(key)
}

func (s *SourceAgent) Remove(key ssh.PublicKey) error {
	return s.agent.Remove(key)
}

func (s *SourceAgent) RemoveAll() error {
	return s.agent.RemoveAll()
}

func (s *SourceAgent) Lock(passphrase []byte) error {
	return s.agent.Lock(passphrase)
}

func (s *SourceAgent) Unlock(passphrase []byte) error {
	return s.agent.Unlock(passphrase)
}

func (s *SourceAgent) Signers() ([]ssh.Signer, error) {
	return s.agent.Signers()
}

func (s *SourceAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	if extAgent, ok := s.agent.(agent.ExtendedAgent); ok {
		return extAgent.Extension(extensionType, contents)
	}
	return nil, agent.ErrExtensionUnsupported
}
