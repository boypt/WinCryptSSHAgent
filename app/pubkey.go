package app

import (
	"context"
	"io"

	"github.com/buptczq/WinCryptSSHAgent/sshagent"
	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh/agent"
)

type PubKeyView struct {
	ag agent.Agent
}

func (s *PubKeyView) Run(ctx context.Context, handler func(conn io.ReadWriteCloser)) error {
	s.ag = ctx.Value("agent").(agent.Agent)
	return nil
}

func (*PubKeyView) AppId() AppId {
	return APP_PUBKEY
}

func (s *PubKeyView) Menu(register func(id AppId, name string, handler func())) {
	register(s.AppId(), "Show Public Keys", s.onClick)
	register(APP_IMPORT, "Import Key…", s.onImport)
}

func (s *PubKeyView) onClick() {
	keys, err := s.ag.List()
	if err != nil {
		utils.MessageBox("Error:", err.Error(), utils.MB_ICONWARNING)
		return
	}

	var pubkey string
	if len(keys) == 0 {
		pubkey = "No Keys"
	} else {
		for _, key := range keys {
			pubkey += key.String() + "\n"
		}
	}

	if utils.MessageBox("Public Keys (OK to copy):", pubkey, utils.MB_OKCANCEL) == utils.IDOK {
		utils.SetClipBoard(pubkey)
	}
}

func (s *PubKeyView) onImport() {
	// Async so the file dialog and passphrase helpers never block the
	// tray event loop; AddKeyFile serializes concurrent imports itself.
	go func() {
		path, ok := utils.OpenFileDialog("Import Private Key")
		if !ok || path == "" {
			return // cancelled
		}
		// Same file→AddedKey logic as startup auto-load; encrypted keys are tried
		// with WCSA_KEY_PASSPHRASE and skipped with a toast on failure.
		_ = sshagent.AddKeyFile(s.ag, path, "Import")
	}()
}
