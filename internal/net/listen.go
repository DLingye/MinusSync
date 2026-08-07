package net

import (
	"crypto/tls"
	"fmt"

	"github.com/MinusSync/internal/protocol"
	"net"
	"time"
)

// Listener accepts incoming MinusSync connections.
type Listener struct {
	inner net.Listener
}

// Listen creates a TCP listener for MinusSync connections.
func Listen(port int) (*Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	inner, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &Listener{inner: inner}, nil
}

// ListenTLS creates a TLS listener for MinusSync connections.
func ListenTLS(port int, cfg *tls.Config) (*Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	inner, err := tls.Listen("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("listen tls %s: %w", addr, err)
	}
	return &Listener{inner: inner}, nil
}

// Accept waits for and returns the next connection.
func (l *Listener) Accept() (*Connection, error) {
	raw, err := l.inner.Accept()
	if err != nil {
		return nil, err
	}

	// Set a deadline for handshake
	raw.SetDeadline(time.Now().Add(30 * time.Second))

	conn, err := acceptConnection(raw)
	if err != nil {
		raw.Close()
		return nil, err
	}

	// Clear deadline
	raw.SetDeadline(time.Time{})

	return conn, nil
}

// acceptConnection handles the server side of a connection handshake.
func acceptConnection(raw net.Conn) (*Connection, error) {
	// Receive client handshake first
	major, minor, err := protocol.ReceiveHandshake(raw)
	if err != nil {
		return nil, fmt.Errorf("receive handshake: %w", err)
	}

	conn := &Connection{
		raw:     raw,
		enc:     protocol.NewEncoder(raw),
		dec:     protocol.NewDecoder(raw),
		version: [2]uint16{major, minor},
	}

	// Send response handshake
	if err := conn.enc.SendHandshake(); err != nil {
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	return conn, nil
}

// Close closes the listener.
func (l *Listener) Close() error {
	return l.inner.Close()
}

// Addr returns the listener's address.
func (l *Listener) Addr() net.Addr {
	return l.inner.Addr()
}
