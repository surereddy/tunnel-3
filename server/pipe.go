package server

import (
	"io"
	"net"

	log "github.com/cosiner/ygo/jsonlog"
)

type buffer struct {
	bufsize int
	c       chan []byte
}

func (b *buffer) Get() []byte {
	select {
	case buf := <-b.c:
		return buf
	default:
		return make([]byte, b.bufsize)
	}
}

func (b *buffer) Put(buf []byte) {
	if cap(buf) != b.bufsize {
		panic("invalid buffer size")
	}
	buf = buf[:b.bufsize]
	select {
	case b.c <- buf:
	default:
	}
}

var bufferPool = buffer{
	bufsize: 8192,
	c:       make(chan []byte, 256),
}

func PipeCloseDst(dst, src net.Conn, logger *log.Logger) {
	buf := bufferPool.Get()
	defer func() {
		dst.Close()
		bufferPool.Put(buf)
	}()

	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		if !isConnClosed(err) {
			logger.Error(log.M{"msg": "pipe error", "err": err.Error()})
		}
	}
}
