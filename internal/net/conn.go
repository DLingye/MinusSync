// Package net provides TCP/TLS network transport for the MinusSync protocol.
package net

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/MinusSync/internal/protocol"
)

// Connection wraps a TCP/TLS connection with protocol encoding/decoding.
type Connection struct {
	raw     io.ReadWriteCloser
	enc     *protocol.Encoder
	dec     *protocol.Decoder
	version [2]uint16
}

// Dial connects to a remote MinusSync server.
func Dial(host string, port int) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	raw, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return newConnection(raw)
}

// DialTLS connects to a remote MinusSync server over TLS.
func DialTLS(host string, port int, cfg *tls.Config) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	raw, err := tls.DialWithDialer(dialer, "tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial tls %s: %w", addr, err)
	}
	return newConnection(raw)
}

// newConnection creates a Connection from a raw connection.
func newConnection(raw io.ReadWriteCloser) (*Connection, error) {
	conn := &Connection{
		raw: raw,
		enc: protocol.NewEncoder(raw),
		dec: protocol.NewDecoder(raw),
	}

	// Send handshake
	if err := conn.enc.SendHandshake(); err != nil {
		raw.Close()
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	// Receive handshake
	major, minor, err := protocol.ReceiveHandshake(raw)
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("receive handshake: %w", err)
	}
	conn.version = [2]uint16{major, minor}

	return conn, nil
}

// Send writes a protocol message.
func (c *Connection) Send(msgType uint16, payload []byte) error {
	return c.enc.Write(msgType, payload)
}

// Recv reads a protocol message.
func (c *Connection) Recv() (uint16, []byte, error) {
	return c.dec.Read()
}

// Close closes the connection.
func (c *Connection) Close() error {
	return c.raw.Close()
}

// Version returns the negotiated protocol version.
func (c *Connection) Version() (major, minor uint16) {
	return c.version[0], c.version[1]
}
