package app

import (
	"context"
	"fmt"
	"github.com/buptczq/WinCryptSSHAgent/utils"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type WSL struct {
	running bool
	help    string
}

func listenUnixSock(filename string) (string, net.Listener, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(home, filename)
	os.Remove(path)
	l, err := net.Listen("unix", path)
	return path, l, err
}

func winPath2Unix(path string) string {
	volumeName := filepath.VolumeName(path)
	vnl := len(volumeName)
	fileName := path[vnl:]
	if vnl == 2 {
		return "/mnt/" + strings.ToLower(string(volumeName[0])) + filepath.ToSlash(fileName)
	} else {
		return filepath.ToSlash(path)
	}
}

func (s *WSL) Run(ctx context.Context, handler func(conn io.ReadWriteCloser)) error {
	fallback := false
	// try to listen unix sock (Win10 1803)
	path, l, err := listenUnixSock(WSL_SOCK)
	if err != nil {
		// fallback to raw tcp
		l, err = net.Listen("tcp", "localhost:0")
		fallback = true
		if err != nil {
			return err
		}
	}
	defer l.Close()

	s.running = true
	if !fallback {
		s.help = fmt.Sprintf("export SSH_AUTH_SOCK=" + winPath2Unix(path))
	} else {
		s.help = fmt.Sprintf("socat UNIX-LISTEN:/tmp/ssh-capi-agent.sock,reuseaddr,fork TCP:localhost:%d &\n", l.Addr().(*net.TCPAddr).Port)
		s.help += "export SSH_AUTH_SOCK=/tmp/ssh-capi-agent.sock"
	}

	wg := new(sync.WaitGroup)
	// context cancelled
	go func() {
		<-ctx.Done()
		l.Close()
		wg.Wait()
	}()

	// loop
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			handler(conn)
			wg.Done()
		}()
	}
}

func (*WSL) AppId() AppId {
	return APP_WSL
}

func (s *WSL) Menu(register func(id AppId, name string, handler func())) {
	register(s.AppId(), "Show "+s.AppId().String()+" Settings", s.onClick)
}

func (s *WSL) onClick() {
	if s.running {
		if utils.MessageBox(s.AppId().FullName()+" (OK to copy):", s.help, utils.MB_OKCANCEL) == utils.IDOK {
			utils.SetClipBoard(s.help)
		}
	} else {
		utils.MessageBox("Error:", s.AppId().String()+" agent doesn't work!", utils.MB_ICONWARNING)
	}
}
