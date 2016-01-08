package server

import (
	"math/rand"
	"net"

	"github.com/cosiner/gohper/net2"
	"github.com/cosiner/tunnel/proxy"
	"github.com/cosiner/ygo/log"
)

func RunMultipleLocal(socks, tunnels []proxy.Proxy, directList, tunnelList, suffixList *SiteList) (sig Signal, err error) {
	sig = NewSignal()
	for _, sock := range socks {
		err = RunLocal(sock, tunnels, sig, directList, tunnelList, suffixList)
		if err != nil {
			break
		}
	}
	if err != nil {
		sig.Close()
		sig = nil
	}
	return
}

type Local struct {
	directList, tunnelList, suffixList *SiteList

	sock    proxy.Proxy
	tunnels []proxy.Proxy

	listener net.Listener
	signal   Signal
}

func RunLocal(sock proxy.Proxy, tunnels []proxy.Proxy, signal Signal, directList, tunnelList, suffixList *SiteList) error {
	ln, err := net2.RetryListen("tcp", sock.Addr(), 5, 1000)
	if err != nil {
		return err
	}

	local := &Local{
		directList: directList,
		tunnelList: tunnelList,
		suffixList: suffixList,

		sock:     sock,
		tunnels:  tunnels,
		listener: ln,
		signal:   signal,
	}

	go local.serve()
	return nil
}

func (l *Local) serve() error {
	for {
		select {
		case <-l.signal:
			return l.listener.Close()
		default:
			conn, err := l.listener.Accept()
			if err != nil {
				return err
			}

			go l.serveConn(conn)
		}
	}
}

func (l *Local) randTunnel() proxy.Proxy {
	return l.tunnels[rand.Intn(len(l.tunnels))]
}

func (l *Local) serveConn(conn net.Conn) {
	var (
		addr   proxy.Addr
		err    error
		remote net.Conn
	)
	defer func() {
		if remote != nil {
			remote.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}()

	conn, addr, err = l.sock.Server(conn)
	if err != nil {
		log.Error("parse socks5 request failed:", err)
		return
	}

	remote, _, err = l.dial(addr)
	if err != nil {
		return
	}

	go PipeCloseDst(conn, remote)
	PipeCloseDst(remote, conn)
	remote = nil
	conn = nil
}

func (l *Local) isDirectConnect(host string) bool {
	if l.suffixList != nil {
		if l.suffixList.Contains(host) {
			return true
		}
	}
	if l.directList != nil {
		if l.directList.Contains(host) {
			return true
		}
	}
	if l.tunnelList != nil {
		if l.tunnelList.Contains(host) {
			return false
		}
	}
	return false
}

func (l *Local) dial(addr proxy.Addr) (conn net.Conn, isTunnel bool, err error) {
	log.Debug(addr.Type, string(addr.Host))
	if host := string(addr.Host); l.isDirectConnect(host) {
		conn, err = net.Dial("tcp", addr.String())
		if err == nil {
			log.Debug("direct connected to", host)
			return conn, false, nil
		}

		log.Error("direct connect to dst server failed, try tunnel:", err)
	}

	// tunnel
	tunnel := l.randTunnel()
	conn, err = net.Dial("tcp", tunnel.Addr())
	if err == nil {
		conn, err = tunnel.Client(conn, addr)
		if err != nil {
			log.Error("request tunnel server failed:", err)
		}
	} else {
		log.Error("dial tunnel server failed:", err)
	}
	return conn, true, err
}

func (l *Local) Mode() string {
	return MODE_LOCAL
}
