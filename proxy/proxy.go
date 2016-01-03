package proxy

import (
	"errors"
	"net"
)

var (
	ErrNoProxy   = errors.New("can't proxy for this connection")
	ErrBadFormat = errors.New("bad format")
)

type Proxy interface {
	// if proxy failed, Client and Server should return original net.Conn
	// rather than nil
	Client(net.Conn, Addr) (net.Conn, error)
	Server(net.Conn) (net.Conn, Addr, error)
	Addr() string
}

func debugForward(conn net.Conn, addr Addr) {
	//	log.Debug("forward for:", conn.RemoteAddr(), "To:", string(addr.Host), addr.Port)
}
