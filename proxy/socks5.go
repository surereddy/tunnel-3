package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/cosiner/gohper/ds/set"
)

var (
	ErrUnsupportedSocksVersion = errors.New("socks version doesn't support")
	ErrNoSupportedMethods      = errors.New("no supported methods")
	ErrAuthFailed              = errors.New("auth failed")
	ErrIllegalAddr             = errors.New("illegal address")
	ErrDomainNameTooLong       = errors.New("domain name too long")
	ErrUserOrPassTooLong       = errors.New("user or password too long")
	ErrNetworkUnreachable      = errors.New("network unreachable")
	ErrHostUnreachable         = errors.New("host unreachable")
	ErrConnRefused             = errors.New("connection refused")
	ErrTTLExpired              = errors.New("ttl expired")
)

const (
	SOCKS_VER byte = 0x05

	AUTH_NOT_REQUIRED byte = 0x00
	_AUTH_GSS_API     byte = 0x01 // unsupported
	AUTH_USER_PASS    byte = 0x03
	AUTH_UNACCEPTABLE byte = 0xff

	CMD_CONNECT        byte = 0x01
	_CMD_BIND          byte = 0x02
	_CMD_UDP_ASSOCIATE      = 0x03

	ADDR_IPV4            byte = 0x01
	ADDR_IPV6            byte = 0x04
	ADDR_DOMAIN_NAME     byte = 0x03
	_MAX_DOMAIN_NAME_LEN      = 255

	USER_PASS_VERIFY_VER     byte = 1
	_MAX_USER_PASS_LEN            = 256
	USER_PASS_VERIFY_SUCCESS      = 0x00
	USER_PASS_VERIFY_FAILED       = 0x01
)

func (up UserPass) Size() int {
	return len(up)
}

type Socks5 struct {
	// common
	userPass       UserPass
	supportMethods set.Bytes // supported methods

	// client
	authRequired bool   // is auth required
	methodReq    []byte // method request

	addr string
}

func cleanMethods(methods []byte) set.Bytes {
	set := make(set.Bytes)
	for _, m := range methods {
		if m == AUTH_USER_PASS || m == AUTH_NOT_REQUIRED {
			set.Put(m)
		}
	}
	return set
}

func NewSocks5(methods []byte, users UserPass, addr string) (*Socks5, error) {
	set := cleanMethods(methods)
	nmethod := set.Size()
	if nmethod == 0 {
		return nil, ErrNoSupportedMethods
	}
	if nmethod == 1 && set.HasKey(AUTH_USER_PASS) && users.Size() == 0 {
		return nil, ErrAuthFailed
	}

	s := &Socks5{
		supportMethods: set,
		authRequired:   !set.HasKey(AUTH_NOT_REQUIRED),
		methodReq:      make([]byte, 2+nmethod),
		userPass:       users,
		addr:           addr,
	}
	s.methodReq[0] = SOCKS_VER
	s.methodReq[1] = byte(nmethod)
	copy(s.methodReq[2:], set.Keys())
	return s, nil
}

func (s *Socks5) Addr() string {
	return s.addr
}

func (s *Socks5) clientVerifyUserPass(conn net.Conn) error {
	// Req:  | Ver 1 | UserLen 1 | User dynamic | PassLen 1 | Pass dynamic |
	// Resp: | Ver 1 | Status  1 |
	user, pass, has := s.userPass.One()
	if !has {
		return ErrAuthFailed
	}

	userLen := len(user)
	passLen := len(pass)
	authReq := make([]byte, 1+1+userLen+1+passLen)
	authReq[0] = USER_PASS_VERIFY_VER
	authReq[1] = byte(userLen)
	copy(authReq[2:], user)
	authReq[2+userLen] = byte(passLen)
	copy(authReq[3+userLen:], pass)

	_, err := conn.Write(authReq)
	if err != nil {
		return err
	}
	var authResp [2]byte
	_, err = io.ReadFull(conn, authResp[:])
	if err != nil {
		return err
	}
	if authResp[0] != USER_PASS_VERIFY_VER || authResp[1] != USER_PASS_VERIFY_SUCCESS {
		return ErrAuthFailed
	}
	return nil
}

func (s *Socks5) clientHandshake(conn net.Conn) (authRequired bool, err error) {
	// Req:  | SocksVer 1 | NMethod 1 | Methods NMethod |
	// Resp: | SocksVer 1 | Method  1 |
	if s.userPass.Size() == 0 && s.authRequired {
		return false, ErrAuthFailed
	}

	_, err = conn.Write(s.methodReq)
	if err != nil {
		return false, err
	}
	var methodResp [2]byte
	_, err = io.ReadFull(conn, methodResp[:])
	if err != nil {
		return false, err
	}
	if methodResp[0] != SOCKS_VER {
		return false, ErrUnsupportedSocksVersion
	}
	switch m := methodResp[1]; m {
	case AUTH_NOT_REQUIRED, AUTH_USER_PASS:
		if s.supportMethods.HasKey(m) {
			return m == AUTH_USER_PASS, nil
		}
	}
	return false, ErrNoSupportedMethods
}

func replyError(code byte) error {
	switch code {
	case 0x03:
		return ErrNetworkUnreachable
	case 0x04:
		return ErrHostUnreachable
	case 0x05:
		return ErrConnRefused
	case 0x06:
		return ErrTTLExpired
	}
	return fmt.Errorf("connection failed: %d", code)
}

const _MAX_CONNECT_DATA_LEN = 4 + 1 + 255 + 2

//  | Ver 1 | CMD 1 | Rsv 0x00 | AddrType 1 | DstAddr dynamic | DstPort 2 |
//  | Ver 1 | REP 1 | Rsv 0x00 | AddrType 1 | DstAddr dynamic | DstPort 2 |
func (s *Socks5) clientConnect(conn net.Conn, addr Addr) error {
	debugForward(conn, addr)

	reqLen := 4 + len(addr.Host) + 2
	if addr.Type == ADDR_DOMAIN_NAME {
		reqLen++
	}
	req := make([]byte, reqLen, _MAX_CONNECT_DATA_LEN)
	req[0] = SOCKS_VER
	req[1] = CMD_CONNECT
	req[2] = 0x00
	req[3] = addr.Type
	if addr.Type == ADDR_DOMAIN_NAME {
		req[4] = byte(len(addr.Host))
	}
	copy(req[reqLen-2-len(addr.Host):], addr.Host)
	binary.BigEndian.PutUint16(req[reqLen-2:], addr.Port)

	_, err := conn.Write(req)
	if err != nil {
		return err
	}
	req = req[:_MAX_CONNECT_DATA_LEN]
	n, err := io.ReadAtLeast(conn, req, 5)
	if err != nil {
		return err
	}
	if req[0] != SOCKS_VER {
		return ErrUnsupportedSocksVersion
	}
	if req[1] != 0x00 {
		return replyError(req[1])
	}
	switch req[3] {
	case ADDR_IPV4:
		req = req[n : 4+net.IPv4len+2]
	case ADDR_IPV6:
		req = req[n : 4+net.IPv6len+2]
	case ADDR_DOMAIN_NAME:
		req = req[n : 5+int(req[4])+2]
	default:
		return errors.New("invalid response format")
	}
	_, err = io.ReadFull(conn, req)
	return err
}

func (s *Socks5) Client(conn net.Conn, addr Addr) (net.Conn, error) {
	authRequired, err := s.clientHandshake(conn)
	if err == nil {
		if authRequired {
			err = s.clientVerifyUserPass(conn)
		}
		if err == nil {
			err = s.clientConnect(conn, addr)
		}
	}
	return conn, err
}

func (s *Socks5) serverHandshake(conn net.Conn) (authRequired bool, err error) {
	var req [257]byte
	n, err := io.ReadAtLeast(conn, req[:], 2)
	if err != nil {
		return false, err
	}
	nmethod := req[1]
	if req[0] != SOCKS_VER || nmethod == 0 {
		conn.Write([]byte{SOCKS_VER, AUTH_UNACCEPTABLE})
		return false, nil
	}
	_, err = io.ReadFull(conn, req[n:2+nmethod])
	if err != nil {
		return false, err
	}
	var selected = AUTH_UNACCEPTABLE
	for _, method := range req[2 : 2+nmethod] {
		if s.supportMethods.HasKey(method) {
			selected = method
		}
	}

	conn.Write([]byte{SOCKS_VER, selected})
	if selected == AUTH_UNACCEPTABLE {
		err = ErrNoProxy
	}
	return selected == AUTH_USER_PASS, err
}

func (s *Socks5) serverVerifyUserPass(conn net.Conn) error {
	var req [512]byte
	un, err := io.ReadAtLeast(conn, req[:], 2)
	if err != nil {
		return err
	}
	if req[0] != USER_PASS_VERIFY_VER {
		conn.Write([]byte{USER_PASS_VERIFY_VER, USER_PASS_VERIFY_FAILED})
		return ErrNoProxy
	}
	userLen := int(req[2])
	pn, err := io.ReadAtLeast(conn, req[un:], userLen+1)
	if err != nil {
		return err
	}
	passLen := int(req[userLen+2])
	_, err = io.ReadFull(conn, req[un+pn:3+userLen+passLen])
	if err != nil {
		return err
	}

	user := req[2 : 2+userLen]
	pass := req[3+userLen : 3+userLen+passLen]
	verified := s.userPass.Verify(string(user), string(pass))
	if verified {
		conn.Write([]byte{USER_PASS_VERIFY_VER, USER_PASS_VERIFY_SUCCESS})
		return nil
	}
	conn.Write([]byte{USER_PASS_VERIFY_VER, USER_PASS_VERIFY_FAILED})
	return ErrNoProxy
}

func (s *Socks5) serverConnectResp(code byte) []byte {
	return []byte{SOCKS_VER, code, 0x00, ADDR_IPV4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func (s *Socks5) serverConnect(conn net.Conn) (a Addr, err error) {
	var req [_MAX_CONNECT_DATA_LEN]byte
	n, err := io.ReadAtLeast(conn, req[:], 5)
	if err != nil {
		return a, err
	}
	if req[0] != SOCKS_VER {
		conn.Write(s.serverConnectResp(0x01))
		return a, ErrNoProxy
	}
	if req[1] != CMD_CONNECT {
		conn.Write(s.serverConnectResp(0x07))
		return a, ErrNoProxy
	}

	var (
		rawLen    int
		addrIndex int
		atyp      = req[3]
	)
	switch atyp {
	case ADDR_IPV4:
		addrIndex = 4
		rawLen = addrIndex + net.IPv4len + 2
	case ADDR_IPV6:
		addrIndex = 4
		rawLen = addrIndex + net.IPv6len + 2
	case ADDR_DOMAIN_NAME:
		addrIndex = 5
		rawLen = addrIndex + int(req[4]) + 2
	default:
		conn.Write(s.serverConnectResp(0x08))
		return a, ErrNoProxy
	}
	_, err = io.ReadFull(conn, req[n:rawLen])
	if err != nil {
		return a, err
	}
	_, err = conn.Write(s.serverConnectResp(0x00))
	if err != nil {
		return a, err
	}

	a, err = NewRawAddr(atyp, req[addrIndex:rawLen-2], binary.BigEndian.Uint16(req[rawLen-2:rawLen]))
	if err == nil {
		a.Raw = req[3:rawLen]
		debugForward(conn, a)
	}
	return
}

func (s *Socks5) Server(conn net.Conn) (c net.Conn, a Addr, err error) {
	var authRequired bool
	authRequired, err = s.serverHandshake(conn)
	if err == nil {
		if authRequired {
			err = s.serverVerifyUserPass(conn)
		}
		if err == nil {
			a, err = s.serverConnect(conn)
		}
	}
	return conn, a, err
}
