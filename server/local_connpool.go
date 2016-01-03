package server

import "net"

const _POOL_SIZE = 8

type ConnPool chan net.Conn

func (p ConnPool) Get() net.Conn {
	select {
	case conn := <-p:
		return conn
	default:
		return nil
	}
}

func (p ConnPool) Put(conn net.Conn) {
	select {
	case p <- conn:
	default:
		conn.Close()
	}
}
