package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/buptczq/WinCryptSSHAgent/utils"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"sync"
)

const (
	maxAgentResponseBytes = 16 << 20
	// XShell Xagent message types.
	agentSignRequest  = 13
	xagentInitRequest = 99
)

type XShell struct {
	cookie string
}

func (s *XShell) Run(ctx context.Context, handler func(conn io.ReadWriteCloser)) error {
	s.cookie = utils.RandomString(7)
	win, err := utils.NewXAgent(s.cookie)
	if err != nil {
		return err
	}
	defer win.Close()

	wg := new(sync.WaitGroup)
	go func() {
		<-ctx.Done()
		win.Close()
		wg.Wait()
	}()

	l := win.Listener()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if err != io.ErrClosedPipe {
				return err
			}
			return nil
		}
		err = xshellHandshake(conn, s.cookie)
		if err != nil {
			println(err.Error())
			conn.Close()
			continue
		}
		wg.Add(1)
		go func(c io.ReadWriteCloser) {
			w := &xshellProxy{conn: c}
			handler(w)
			wg.Done()
		}(conn)
	}
}

func (*XShell) AppId() AppId {
	return APP_XSHELL
}

func (s *XShell) Menu(register func(id AppId, name string, handler func())) {
}

type initAgentMsg struct {
	Flag   uint32 `sshtype:"99"`
	Length uint32
	Cookie []byte `ssh:"rest"`
}

type initAgentRepMsg struct {
	Flag uint32 `sshtype:"99"`
}

func xshellHandshake(conn net.Conn, cookie string) error {
	var length [4]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return err
	}
	l := binary.BigEndian.Uint32(length[:]) + 4
	if l > maxAgentResponseBytes {
		return fmt.Errorf("xagent: request too large: %d", l)
	}

	req := make([]byte, l)
	if _, err := io.ReadFull(conn, req); err != nil {
		return err
	}
	if req[0] != xagentInitRequest {
		return fmt.Errorf("xagent: unknown opcode: %d", req[0])
	}

	var initMsg initAgentMsg
	if err := ssh.Unmarshal(req, &initMsg); err != nil {
		return err
	}
	if int(initMsg.Length) != len(cookie) {
		return fmt.Errorf("xagent: invalid cookie length")
	}
	// Newer XShell (8 build 0119+) may append extra data (e.g. a key name)
	// after the cookie in the handshake frame; keep only the first Length bytes.
	// This extended handshake layout first appeared in XShell 8 build 0119.
	if len(initMsg.Cookie) > int(initMsg.Length) {
		initMsg.Cookie = initMsg.Cookie[:initMsg.Length]
	}

	cookieRemain := make([]byte, int(initMsg.Length)-len(initMsg.Cookie))
	if _, err := io.ReadFull(conn, cookieRemain); err != nil {
		return err
	}
	cookieReq := string(initMsg.Cookie) + string(cookieRemain)
	if cookieReq != cookie {
		return fmt.Errorf("xagent: invalid cookie")
	}
	var repMsg initAgentRepMsg
	repMsg.Flag = initMsg.Flag
	rep := ssh.Marshal(&repMsg)
	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(rep))); err != nil {
		return err
	}
	if _, err := buf.Write(rep); err != nil {
		return err
	}
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

type xshellProxy struct {
	conn    io.ReadWriteCloser
	buf     []byte
	wlength int
	wbuf    []byte
}

type signRequestAgentMsg struct {
	KeyBlob []byte `sshtype:"13"`
	Data    []byte
	Flags   uint32
	Rest    []byte `ssh:"rest"`
}

func (s *xshellProxy) Read(p []byte) (n int, err error) {
	if len(s.buf) > 0 {
		n := copy(p, s.buf)
		s.buf = s.buf[n:]
		return n, nil
	}
	var length [4]byte
	if _, err := io.ReadFull(s.conn, length[:]); err != nil {
		println("xagent proxy read err:", err.Error())
		return 0, err
	}
	l := binary.BigEndian.Uint32(length[:])
	if l == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if l > maxAgentResponseBytes {
		err := fmt.Errorf("xagent: request too large: %d", l)
		println(err.Error())
		return 0, err
	}

	s.buf = make([]byte, 4+l, 8+l)
	if _, err := io.ReadFull(s.conn, s.buf[4:]); err != nil {
		println("xagent proxy read err:", err.Error())
		return 0, err
	}

	// Canonicalize sign requests (type 13) before handing them to the standard
	// agent server. A sign request body is laid out as:
	//   type(1 byte) | KeyBlob(4-byte length + blob) |
	//   Data(4-byte length + data) | Flags(4 bytes)
	// Two XShell deviations are handled here:
	//   - newer builds append trailing bytes after Flags: truncate them;
	//   - older builds omit Flags entirely: pad with zero Flags instead.
	// Every step below is bounds-checked so a short or malformed frame sets
	// ok=false (and is passed through untouched) instead of panicking.
	if s.buf[4] == agentSignRequest {
		data := s.buf[4:]
		bodyLen := uint32(len(data))

		totalLen := uint32(0)
		ok := true
		// Field 1: message type, 1 byte (always 13 on this path).
		if bodyLen < 1 {
			ok = false
		} else {
			totalLen = 1
		}
		// Field 2: KeyBlob, a length-prefixed string: 4-byte length + blob.
		if ok && totalLen+4 > bodyLen {
			ok = false
		} else if ok {
			kbLen := binary.BigEndian.Uint32(data[totalLen:])
			if totalLen+4+kbLen > bodyLen {
				ok = false
			} else {
				totalLen += 4 + kbLen
			}
		}
		// Field 3: Data (the bytes to sign), also length-prefixed:
		// 4-byte length + data.
		if ok && totalLen+4 > bodyLen {
			ok = false
		} else if ok {
			dataLen := binary.BigEndian.Uint32(data[totalLen:])
			if totalLen+4+dataLen > bodyLen {
				ok = false
			} else {
				totalLen += 4 + dataLen
			}
		}
		// Field 4: Flags, 4 bytes. Optional: present on newer builds,
		// absent on older ones.
		hasFlags := false
		if ok && totalLen+4 <= bodyLen {
			hasFlags = true
			totalLen += 4
		}

		if ok && totalLen > 0 {
			// out covers the 4-byte frame length prefix plus the canonical
			// body; anything beyond it is XShell trailing bytes.
			out := s.buf[:totalLen+4]
			var req signRequestAgentMsg
			if err := ssh.Unmarshal(out[4:], &req); err == nil {
				l = totalLen
				s.buf = out
			} else if !hasFlags {
				// Older build without Flags: append 4 zero bytes as Flags
				// and retry; on success grow the buffer by those 4 bytes.
				padded := make([]byte, len(out[4:])+4)
				copy(padded, out[4:])
				if err2 := ssh.Unmarshal(padded, &req); err2 == nil {
					newBuf := make([]byte, 4+int(totalLen)+4)
					copy(newBuf[:4], s.buf[:4])
					copy(newBuf[4:], s.buf[4:4+int(totalLen)])
					s.buf = newBuf
					l = totalLen + 4
				}
			}
		}
	}
	binary.BigEndian.PutUint32(s.buf, l)
	n = copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func (s *xshellProxy) Write(p []byte) (n int, err error) {
	// xshell treats TCP as a message-oriented connection
	// this piece of sh*t code is in order to be compatible with xshell
	if s.wlength == 0 {
		if len(p) != 4 {
			return 0, fmt.Errorf("xagent proxy: invalid write status")
		}
		s.wlength = int(binary.BigEndian.Uint32(p)) + 4
		s.wbuf = append(s.wbuf, p...)
	} else {
		s.wbuf = append(s.wbuf, p...)
		if len(s.wbuf) == s.wlength {
			s.wlength = 0
			_, err := s.conn.Write(s.wbuf)
			if err != nil {
				println("xagent proxy write err:", err.Error())
				return 0, err
			}
			s.wbuf = nil
		} else if len(s.wbuf) > s.wlength {
			return 0, fmt.Errorf("xagent proxy: invalid write length")
		}
	}
	return len(p), nil
}

func (s *xshellProxy) Close() error {
	return s.conn.Close()
}
