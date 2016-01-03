package server

import (
	"net"

	"io"

	"github.com/cosiner/gohper/net2"
	"github.com/cosiner/tunnel/proxy"
	"github.com/cosiner/ygo/log"
)

func RunMultipleRemote(tunnels []proxy.Proxy) (sig Signal, err error) {
	sig = NewSignal()
	for _, tunnel := range tunnels {
		err = RunRemote(tunnel, sig)
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

type Remote struct {
	tunnel proxy.Proxy

	listener net.Listener
	signal   Signal
}

func RunRemote(tunnel proxy.Proxy, signal Signal) error {
	ln, err := net2.RetryListen("tcp", tunnel.Addr(), 5, 1000)
	if err != nil {
		return err
	}

	r := &Remote{
		tunnel:   tunnel,
		signal:   signal,
		listener: ln,
	}
	go r.serve()
	return nil
}

func (r *Remote) serve() error {
	for {
		select {
		case <-r.signal:
			return r.listener.Close()
		default:
			conn, err := r.listener.Accept()
			if err != nil {
				return err
			}

			go r.serveConn(conn)
		}
	}
}

func (r *Remote) serveConn(conn net.Conn) {
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

	conn, addr, err = r.tunnel.Server(conn)
	if err != nil {
		if err != io.EOF && !isConnClosed(err) {
			log.Error("parse tunnel request failed:", err)
		}
		return
	}

	addrStr := addr.String()
	remote, err = net.Dial("tcp", addrStr)
	if err != nil {
		log.Errorf("connect to remote server %s failed: %s\n", addrStr, err.Error())
		return
	}

	go PipeCloseDst(remote, conn)
	PipeCloseDst(conn, remote)
	conn = nil
	remote = nil
}

func (r *Remote) Mode() string {
	return MODE_REMOTE
}
