package sshagent

import (
	"golang.org/x/crypto/ssh/agent"
	"io"
)

type Server struct {
	Agent agent.Agent
}

func (s *Server) SSHAgentHandler(conn io.ReadWriteCloser) {
	s.SSHAgentHandlerWithSource("")(conn)
}

func (s *Server) SSHAgentHandlerWithSource(source string) func(conn io.ReadWriteCloser) {
	return func(conn io.ReadWriteCloser) {
		defer conn.Close()
		if s.Agent == nil {
			return
		}
		var ag agent.Agent = s.Agent
		if source != "" {
			ag = NewSourceAgent(s.Agent, source)
		}
		err := agent.ServeAgent(ag, conn)
		if err != nil && err != io.EOF {
			println(err.Error())
		}
	}
}

