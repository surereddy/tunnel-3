package server

import (
	"math/rand"
	"net"

	"github.com/cosiner/gohper/net2"
	"github.com/cosiner/tunnel/proxy"
	log "github.com/cosiner/ygo/jsonlog"
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

	log *log.Logger
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
		log:      log.Derive("Local", sock.Addr()),
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
		l.log.Warn(log.M{"msg": "parse socks5 request failed:", "err": err.Error()})
		return
	}

	remote, _, err = l.dial(addr)
	if err != nil {
		return
	}

	go PipeCloseDst(conn, remote, l.log)
	PipeCloseDst(remote, conn, l.log)
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
	host := string(addr.Host)
	l.log.Info(log.M{"addr_type": addr.Type, "host": host, "port": addr.Port})
	if host := string(addr.Host); l.isDirectConnect(host) {
		conn, err = net.Dial("tcp", addr.String())
		if err == nil {
			if l.log.IsDebugEnable() {
				l.log.Debug(log.M{"connect_mode": "direct", "host": host})
			}
			return conn, false, nil
		}

		l.log.Error(log.M{"msg": "direct connect failed, try tunnel.", "host": host, "err": err.Error()})
	}

	// tunnel
	tunnel := l.randTunnel()
	conn, err = net.Dial("tcp", tunnel.Addr())
	if err == nil {
		conn, err = tunnel.Client(conn, addr)
		if err != nil {
			l.log.Error(log.M{"msg": "tunnel handshake failed", "addr": tunnel.Addr(), "err": err.Error()})
		}
	} else {
		l.log.Error(log.M{"msg": "connect tunnel server failed", "addr": tunnel.Addr(), "err": err.Error()})
	}
	return conn, true, err
}

func (l *Local) Mode() string {
	return MODE_LOCAL
}
