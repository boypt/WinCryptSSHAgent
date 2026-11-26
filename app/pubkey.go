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
	register(APP_IMPORT_CAPI, "Add Windows Certificate Key…", s.onImportCAPI)
	register(APP_CLEAR_KEYS, "Clear Loaded Keys and Saved Selections…", s.onClearKeys)
}

func (s *PubKeyView) onClearKeys() {
	clearer, ok := s.ag.(interface{ ClearSavedKeys() error })
	if !ok {
		utils.MessageBox("Clear SSH Keys", "This agent does not support clearing saved key selections.", utils.MB_ICONWARNING)
		return
	}
	if utils.MessageBox("Clear SSH Keys", "Remove all loaded keys and forget saved key selections?", utils.MB_YESNO|utils.MB_ICONWARNING) != utils.IDYES {
		return
	}
	go func() {
		if err := clearer.ClearSavedKeys(); err != nil {
			utils.MessageBoxForeground("Clear SSH Keys", err.Error(), utils.MB_ICONWARNING)
		}
	}()
}

func (s *PubKeyView) onImportCAPI() {
	go func() {
		importer, ok := s.ag.(interface{ ImportCertificate() error })
		if !ok {
			utils.MessageBox("Add Windows Certificate Key", "Windows certificate-store keys are disabled.", utils.MB_ICONWARNING)
			return
		}
		if err := importer.ImportCertificate(); err != nil {
			utils.MessageBoxForeground("Add Windows Certificate Key", err.Error(), utils.MB_ICONWARNING)
		}
	}()
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
		generation := sshagent.SavedKeyGeneration()
		path, ok := utils.OpenFileDialog("Import Private Key")
		if !ok || path == "" {
			return // cancelled
		}
		// Import only the selected file and remember its path for future starts.
		if err := sshagent.ImportKeyFile(s.ag, path, "Import", generation); err != nil {
			utils.MessageBoxForeground("Import Key", err.Error(), utils.MB_ICONWARNING)
		}
	}()
}
