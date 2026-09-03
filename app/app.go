package app

import (
	"context"
	"io"
)

const (
	WSL_SOCK    = "wincrypt-wsl.sock"
	CYGWIN_SOCK = "wincrypt-cygwin.sock"
	NAMED_PIPE  = "\\\\.\\pipe\\openssh-ssh-agent"
	APP_CYGWIN  = iota
	APP_WSL
	APP_WINSSH
	APP_HYPERV
	APP_SECURECRT
	APP_PAGEANT
	APP_XSHELL
	APP_PUBKEY
	APP_WSL2
	MENU_QUIT
	MENU_CONFIRM_MANUAL
	MENU_CONFIRM_AUTO
)

type Application interface {
	AppId() AppId
	Run(ctx context.Context, handler func(conn io.ReadWriteCloser)) error
	Menu(func(id AppId, name string, handler func()))
}

type AppId int

var appIdToName = map[AppId]string{
	APP_CYGWIN:    "Cygwin",
	APP_WSL:       "WSL",
	APP_WINSSH:    "WinSSH",
	APP_SECURECRT: "SecureCRT",
	APP_PAGEANT:   "Pageant",
	APP_XSHELL:    "XShell",
	APP_HYPERV:    "Hyper-V",
	APP_PUBKEY:    "PubKey",
	APP_WSL2:      "WSL2",
}

var appIdToFullName = map[AppId]string{
	APP_CYGWIN:    "Cygwin (MinGW64 & MSYS2)",
	APP_WSL:       "Windows Subsystem for Linux",
	APP_WINSSH:    "Windows OpenSSH",
	APP_SECURECRT: "SecureCRT",
	APP_PAGEANT:   "Pageant",
	APP_XSHELL:    "XShell",
	APP_HYPERV:    "Hyper-V / WSL2",
	APP_PUBKEY:    "Public Keys",
	APP_WSL2:      "Windows Subsystem for Linux (WSL2)",
}

func (id AppId) String() string {
	return appIdToName[id]
}

func (id AppId) FullName() string {
	return appIdToFullName[id]
}
