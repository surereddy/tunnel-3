package proxy

import (
	"encoding/binary"
	"errors"
	"io"
	"net"

	"github.com/cosiner/ygo/log"
)

type (
	Tunnel struct {
		addr string

		originCipher *Cipher
	}
)

var cipherMetas = make(map[string]*CipherMeta)

func init() {
	methods := cipherMethods{}
	cipherMetas["rc4-128-md5"] = NewCipherMeta(16, 16, methods.NewRc4Md5Stream)
	cipherMetas["aes-128-cfb"] = NewCipherMeta(16, 16, methods.NewAESStream)
	cipherMetas["aes-256-cfb"] = NewCipherMeta(32, 16, methods.NewAESStream)
}

func NewTunnel(method, key, addr string) (Proxy, error) {
	meta, has := cipherMetas[method]
	if !has {
		return nil, errors.New("encrypt method not found:" + method)
	}
	if key == "" {
		return nil, errors.New("empty key is not allowed")
	}

	return &Tunnel{
		addr:         addr,
		originCipher: NewCipher([]byte(key), meta),
	}, nil
}

func (t *Tunnel) Addr() string {
	return t.addr
}

// | AddrType 1 | Addr dynamic | Port 2 |
func (t *Tunnel) clientRequest(conn net.Conn, addr Addr) error {
	switch addr.Type {
	case ADDR_IPV4, ADDR_IPV6, ADDR_DOMAIN_NAME:
	default:
		log.Error("unsupported addr type:", addr.Type)
		return ErrNoProxy
	}
	debugForward(conn, addr)
	_, err := conn.Write(addr.ToRaw())
	return err
}

func (t *Tunnel) Client(conn net.Conn, addr Addr) (net.Conn, error) {
	conn = NewConn(conn, t.originCipher.Copy())
	return conn, t.clientRequest(conn, addr)
}

func (t *Tunnel) serverRequest(conn net.Conn) (a Addr, err error) {
	var rawAddr [259]byte
	_, err = io.ReadFull(conn, rawAddr[:2])
	if err != nil {
		return a, err
	}

	a.Type = rawAddr[0]
	var rawLen int
	var addrIndex int
	switch a.Type {
	case ADDR_IPV4:
		addrIndex = 1
		rawLen = addrIndex + net.IPv4len + 2
	case ADDR_IPV6:
		addrIndex = 1
		rawLen = addrIndex + net.IPv6len + 2
	case ADDR_DOMAIN_NAME:
		addrIndex = 2
		rawLen = addrIndex + int(rawAddr[1]) + 2
	default:
		log.Error("unsupported addr type", a.Type)
		return a, ErrNoProxy
	}

	_, err = io.ReadFull(conn, rawAddr[2:rawLen])
	if err != nil {
		return a, err
	}
	a.Host = rawAddr[addrIndex : rawLen-2]
	a.Port = binary.BigEndian.Uint16(rawAddr[rawLen-2 : rawLen])
	debugForward(conn, a)
	return a, nil
}

func (t *Tunnel) Server(conn net.Conn) (c net.Conn, a Addr, err error) {
	conn = NewConn(conn, t.originCipher.Copy())
	a, err = t.serverRequest(conn)
	return conn, a, err
}
