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
	agentSignRequest      = 13
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
	// Read the 4-byte length prefix. Different XShell versions use different
	// semantics for this value (legacy builds send a small value and split the
	// cookie across the frame boundary; newer builds send the full body length).
	// We therefore only use it to decide how many bytes to read, then parse the
	// fixed-layout payload: Flag(4) + Length(4) + Cookie(Length).
	var prefix [4]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return err
	}
	a := binary.BigEndian.Uint32(prefix[:])

	// The length prefix is a "body length - 4" (i.e. frame body = a+4) for all
	// observed XShell versions; read the full frame body accordingly.
	var bodyLen int = int(a) + 4
	if bodyLen < 5 || bodyLen > maxAgentResponseBytes {
		return fmt.Errorf("xagent: request too large: %d", bodyLen)
	}

	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return err
	}
	if body[0] != 99 {
		return fmt.Errorf("xagent: unknown opcode: %d", body[0])
	}

	// Detect layout: older XShell sends [type][length][cookie...],
	// newer XShell sends [type][extra(4)][length][cookie...] (extra=1 seen).
	lengthOff := 1
	cookieOff := 5
	if v := binary.BigEndian.Uint32(body[1:5]); int(v) != len(cookie) {
		if len(body) >= 9 {
			if v2 := binary.BigEndian.Uint32(body[5:9]); int(v2) == len(cookie) {
				lengthOff = 5
				cookieOff = 9
			}
		}
	}
	if len(body) < cookieOff {
		return fmt.Errorf("xagent: short body")
	}
	length := binary.BigEndian.Uint32(body[lengthOff : lengthOff+4])
	if int(length) != len(cookie) {
		return fmt.Errorf("xagent: invalid cookie length (at off %d, got %d)", lengthOff, length)
	}

	// Cookie bytes may be (partially) in body[cookieOff:] and (partially) in
	// the remainder of the stream. Take at most `length` bytes from body, then
	// read any missing bytes from the connection.
	captured := body[cookieOff:]
	if len(captured) > int(length) {
		captured = captured[:length]
	}
	var cookieRemain []byte
	if len(captured) < int(length) {
		cookieRemain = make([]byte, int(length)-len(captured))
		if _, err := io.ReadFull(conn, cookieRemain); err != nil {
			return err
		}
	}

	cookieReq := string(captured) + string(cookieRemain)
	if cookieReq != cookie {
		return fmt.Errorf("xagent: invalid cookie")
	}

	var repMsg initAgentRepMsg
	repMsg.Flag = 99
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

	// sign: XShell may append trailing bytes (or omit Flags on older builds).
	// Compute the canonical length defensively; never panic on short data.
	if s.buf[4] == agentSignRequest {
		data := s.buf[4:]
		bodyLen := uint32(len(data))

		totalLen := uint32(0)
		ok := true
		if bodyLen < 1 {
			ok = false
		} else {
			totalLen = 1
		}
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
		hasFlags := false
		if ok && totalLen+4 <= bodyLen {
			hasFlags = true
			totalLen += 4
		}

		if ok && totalLen > 0 {
			out := s.buf[:totalLen+4]
			var req signRequestAgentMsg
			if err := ssh.Unmarshal(out[4:], &req); err == nil {
				l = totalLen
				s.buf = out
			} else if !hasFlags {
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
