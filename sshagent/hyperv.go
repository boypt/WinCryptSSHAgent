package sshagent

import (
	"context"
	"fmt"
	"time"

	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const hvDialTimeout = 5 * time.Second

type HVAgent struct {
}

func NewHVAgent() *HVAgent {
	return &HVAgent{}
}

func (s *HVAgent) List() ([]*agent.Key, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hvDialTimeout)
	defer cancel()
	conn, err := utils.ConnectHyperV(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	proxy := agent.NewClient(conn)
	return proxy.List()
}

func (s *HVAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hvDialTimeout)
	defer cancel()
	conn, err := utils.ConnectHyperV(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	proxy := agent.NewClient(conn)
	return proxy.Sign(key, data)
}

func (s *HVAgent) Add(key agent.AddedKey) error {
	return fmt.Errorf("implement me")
}

func (s *HVAgent) Remove(key ssh.PublicKey) error {
	return fmt.Errorf("implement me")
}

func (s *HVAgent) RemoveAll() error {
	return fmt.Errorf("implement me")
}

func (s *HVAgent) Lock(passphrase []byte) error {
	return fmt.Errorf("implement me")
}

func (s *HVAgent) Unlock(passphrase []byte) error {
	return fmt.Errorf("implement me")
}

func (s *HVAgent) Signers() ([]ssh.Signer, error) {
	return nil, fmt.Errorf("implement me")
}
