package msyncd

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/MinusSync/internal/net"
	"github.com/MinusSync/internal/protocol"
)

// Server is the msyncd daemon server.
type Server struct {
	cfg     *Config
	ln      *net.Listener
	repos   map[string]*RepoManager
	mu      sync.RWMutex
	quit    chan struct{}
	stopped bool
}

// NewServer creates a new msyncd server.
func NewServer(cfg *Config) (*Server, error) {
	s := &Server{
		cfg:   cfg,
		repos: make(map[string]*RepoManager),
		quit:  make(chan struct{}),
	}

	// Initialize repository managers
	for _, repoCfg := range cfg.Repositories {
		rm, err := NewRepoManager(repoCfg.Path, repoCfg.ReadOnly)
		if err != nil {
			return nil, fmt.Errorf("init repo %s: %w", repoCfg.Name, err)
		}
		s.repos[repoCfg.Name] = rm
	}

	return s, nil
}

// Start begins listening for connections.
func (s *Server) Start() error {
	var err error
	if s.cfg.Listen.TLS.Enabled {
		s.ln, err = net.ListenTLS(s.cfg.Listen.Port, nil)
	} else {
		s.ln, err = net.Listen(s.cfg.Listen.Port)
	}
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer s.ln.Close()

	log.Printf("Listening on %s", s.ln.Addr())

	for {
		select {
		case <-s.quit:
			return nil
		default:
		}

		conn, err := s.ln.Accept()
		if err != nil {
			if s.stopped {
				return nil
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.stopped = true
	close(s.quit)
	if s.ln != nil {
		s.ln.Close()
	}
}

// handleConnection processes a single client connection.
func (s *Server) handleConnection(conn *net.Connection) {
	defer conn.Close()

	// Authentication (if required)
	if s.cfg.Auth.Method == "token" {
		if !s.authenticate(conn) {
			conn.Send(protocol.MsgError, protocol.EncodeError(protocol.ErrAuthFailed, "authentication failed"))
			return
		}
	}

	// Main protocol loop
	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return // Connection closed or error
		}

		switch msgType {
		case protocol.MsgListRefs:
			s.handleListRefs(conn, payload)

		case protocol.MsgFetchRequest:
			s.handleFetch(conn)

		case protocol.MsgPushRequest:
			s.handlePush(conn)

		case protocol.MsgSearchReq:
			s.handleSearch(conn, payload)

		default:
			conn.Send(protocol.MsgError, protocol.EncodeError(protocol.ErrBadRequest,
				fmt.Sprintf("unknown message type: %s", protocol.MessageName(msgType))))
		}
	}
}

func (s *Server) authenticate(conn *net.Connection) bool {
	conn.Send(protocol.MsgAuthRequest, protocol.EncodeAuthRequest([]string{"token"}))

	msgType, payload, err := conn.Recv()
	if err != nil || msgType != protocol.MsgAuthResponse {
		return false
	}

	method, data, err := protocol.DecodeAuthResponse(payload)
	if err != nil || method != "token" {
		return false
	}

	// Parse "user:token"
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return false
	}

	for _, t := range s.cfg.Auth.Tokens {
		if t.User == parts[0] && t.Token == parts[1] {
			return true
		}
	}
	return false
}

func (s *Server) handleListRefs(conn *net.Connection, payload []byte) {
	// For a single-repo server, advertise all refs
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, rm := range s.repos {
		refs, err := rm.ListRefs()
		if err != nil {
			continue
		}
		for name, h := range refs {
			payload := protocol.EncodeRefLine(name, h)
			conn.Send(protocol.MsgRefLine, payload)
		}
	}

	conn.Send(protocol.MsgStreamEnd, nil)
}

func (s *Server) handleFetch(conn *net.Connection) {
	// Read wants/haves and send pack
	var wants []byte
	var haves [][]byte

	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return
		}
		switch msgType {
		case protocol.MsgWanted:
			wants = payload
		case protocol.MsgHave:
			haves = append(haves, payload)
		case protocol.MsgFetchDone:
			goto sendPack
		}
	}

sendPack:
	// For now, send a simple response
	_ = wants
	_ = haves
	conn.Send(protocol.MsgPackHeader, []byte{0, 0, 0, 0})
	conn.Send(protocol.MsgStreamEnd, nil)
}

func (s *Server) handlePush(conn *net.Connection) {
	// Read update ref request
	msgType, payload, err := conn.Recv()
	if err != nil || msgType != protocol.MsgUpdateRef {
		conn.Send(protocol.MsgError, protocol.EncodeError(protocol.ErrBadRequest, "expected UPDATE_REF"))
		return
	}

	_, _, _, err = protocol.DecodeUpdateRef(payload)
	if err != nil {
		conn.Send(protocol.MsgError, protocol.EncodeError(protocol.ErrBadRequest, err.Error()))
		return
	}

	// Request pack from client
	conn.Send(protocol.MsgWantPack, nil)

	// Receive pack
	for {
		msgType, _, err := conn.Recv()
		if err != nil {
			return
		}
		if msgType == protocol.MsgStreamEnd {
			break
		}
	}

	conn.Send(protocol.MsgOK, nil)
}

func (s *Server) handleSearch(conn *net.Connection, payload []byte) {
	// Search not yet implemented for server
	conn.Send(protocol.MsgStreamEnd, nil)
}
