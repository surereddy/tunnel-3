package server

import (
	"math/rand"
	"net"

	"github.com/cosiner/gohper/net2"
	"github.com/cosiner/tunnel/proxy"
	"github.com/cosiner/ygo/log"
)

func RunMultipleLocal(socks, tunnels []proxy.Proxy, list *SiteList, poolSize int) (sig Signal, err error) {
	sig = NewSignal()
	var pool ConnPool

	if poolSize > 0 {
		pool = make(ConnPool, len(tunnels)*poolSize)
	}
	for _, sock := range socks {
		err = RunLocal(sock, tunnels, sig, list, pool)
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
	list *SiteList

	sock    proxy.Proxy
	tunnels []proxy.Proxy

	listener net.Listener
	signal   Signal

	pool ConnPool
}

func RunLocal(sock proxy.Proxy, tunnels []proxy.Proxy, signal Signal, list *SiteList, pool ConnPool) error {
	ln, err := net2.RetryListen("tcp", sock.Addr(), 5, 1000)
	if err != nil {
		return err
	}

	local := &Local{
		list:     list,
		sock:     sock,
		tunnels:  tunnels,
		listener: ln,
		signal:   signal,
		pool:     pool,
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

	var isTunnel bool
	remote, isTunnel, err = l.dial(addr)
	if err != nil {
		return
	}

	if l.pool == nil {
		go PipeCloseDst(conn, remote)
		PipeCloseDst(remote, conn)
	} else {
		go Pipe(conn, remote, false, true, true)
		if !Pipe(remote, conn, true, false, !isTunnel) {
			l.pool.Put(remote)
		}
	}
	remote = nil
	conn = nil
}

func (l *Local) dial(addr proxy.Addr) (conn net.Conn, isTunnel bool, err error) {
	log.Debug(addr.Type, string(addr.Host))
	if l.list != nil {
		isWhite := l.list.IsWhiteMode()
		inList := l.list.Contains(string(addr.Host))
		if (isWhite && inList) || (!isWhite && !inList) {
			// direct
			conn, err = net.Dial("tcp", addr.String())
			if err == nil {
				return conn, false, nil
			}

			log.Error("direct connect to dst server failed, try tunnel:", err)
		}
	}

	// tunnel
	tunnel := l.randTunnel()
	if l.pool != nil {
		conn = l.pool.Get()
	}
	if conn == nil {
		conn, err = net.Dial("tcp", tunnel.Addr())
	}
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