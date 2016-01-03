package proxy

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

type Addr struct {
	Type byte
	Host []byte
	Port uint16

	Raw []byte
}

func NewAddr(typ byte, addr string) (a Addr, err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return a, err
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return a, err
	}
	switch typ {
	case ADDR_IPV6, ADDR_IPV4:
		ip := net.ParseIP(host)
		if ip == nil || (typ == ADDR_IPV4 && len(ip) != 4) || (typ == ADDR_IPV6 && len(ip) != 6) {
			return a, ErrIllegalAddr
		}
		return NewRawAddr(typ, ip, uint16(p))
	case ADDR_DOMAIN_NAME:
		return NewRawAddr(typ, []byte(host), uint16(p))
	}
	return a, ErrIllegalAddr
}

func NewRawAddr(typ byte, addr []byte, port uint16) (a Addr, err error) {
	if port == 0 {
		return a, ErrIllegalAddr
	}
	switch typ {
	case ADDR_IPV4, ADDR_IPV6:
		if (typ == ADDR_IPV4 && len(addr) != net.IPv4len) || (typ == ADDR_IPV4 && len(addr) != net.IPv6len) {
			return a, ErrIllegalAddr
		}
	case ADDR_DOMAIN_NAME:
		if len(addr) > _MAX_DOMAIN_NAME_LEN-1 {
			return a, ErrDomainNameTooLong
		}
	default:
		return a, ErrIllegalAddr
	}

	return Addr{
		Type: typ,
		Host: addr,
		Port: port,
	}, nil
}

func (a *Addr) ToRaw() []byte {
	if len(a.Raw) != 0 {
		return a.Raw
	}

	addrLen := len(a.Host)
	rawLen := 1 + len(a.Host) + 2
	if a.Type == ADDR_DOMAIN_NAME {
		rawLen++
	}

	a.Raw = make([]byte, rawLen)
	a.Raw[0] = a.Type

	addrIndex := 1
	if a.Type == ADDR_DOMAIN_NAME {
		a.Raw[1] = byte(addrLen)
		addrIndex = 2
	}
	copy(a.Raw[addrIndex:rawLen-2], a.Host)
	binary.BigEndian.PutUint16(a.Raw[rawLen-2:], a.Port)
	return a.Raw
}

func (a *Addr) String() string {
	return fmt.Sprintf("%s:%d", string(a.Host), a.Port)
}
