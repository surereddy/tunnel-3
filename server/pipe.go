package server

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/cosiner/ygo/log"
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


func PipeCloseDst(dst, src net.Conn) {
	buf := bufferPool.Get()
	defer func() {
		dst.Close()
		bufferPool.Put(buf)
	}()

	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		if !isConnClosed(err) {
			log.Error("pipe error:", err)
		}
	}
}

func Pipe(dst, src net.Conn, isDstTunnel, isSrcTunnel, closeDst bool) bool {
	buf := bufferPool.Get()
	defer func() {
		if closeDst {
			dst.Close()
		}
		bufferPool.Put(buf)
	}()

	if isSrcTunnel == isDstTunnel {
		pipeSameType(dst, src, buf)
		return !isDstTunnel || closeDst
	}
	if isDstTunnel {
		return pipeDstTunnel(dst, src, buf) || closeDst
	}
	return pipeSrcTunnel(dst, src, buf) || closeDst
}

func pipeSameType(dst, src net.Conn, buf []byte) {
	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		log.Error("pipe error:", err)
	}
}

// | 31bit size + 1bit isEnd | data
func pipeDstTunnel(dst, src net.Conn, buf []byte) bool {
	var err error
	for {
		nr, er := src.Read(buf[2:])
		if nr > 0 {
			binary.BigEndian.PutUint16(buf, uint16(nr)<<1)
			nw, ew := dst.Write(buf[0 : nr+2])
			if ew != nil {
				err = ew
				break
			}
			if nr+2 != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			err = er
			break
		}
	}
	if err == io.EOF {
		binary.BigEndian.PutUint16(buf, 1)
		n, ew := dst.Write(buf[:2])
		if ew != nil {
			err = ew
		} else if n != 2 {
			err = io.ErrShortWrite
		}
	}

	if err != nil && err != io.EOF && !isConnClosed(err) {
		log.Error("pipe error:", err)
	}
	return false
}

// | 31bit size + 1bit isEnd | data
func pipeSrcTunnel(dst, src net.Conn, buf []byte) bool {
	var (
		err     error
		bufsize = cap(buf)
	)

OUTER:
	for {
		_, err = io.ReadFull(src, buf[:2])
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
		head := binary.BigEndian.Uint16(buf[:2])
		if head&1 != 0 {
			break
		}

		size := int(head >> 1)
		for size > 0 {
			nr := size
			if nr > bufsize {
				nr = bufsize
			}
			size -= nr

			_, er := io.ReadFull(src, buf[:nr])
			if er != nil {
				err = er
				break OUTER
			}

			nw, ew := dst.Write(buf[:nr])
			if ew != nil {
				err = ew
			}
			if nw != nr {
				err = io.ErrShortWrite
				break OUTER
			}
		}
	}

	if err != nil && !isConnClosed(err) {
		log.Error("pipe error:", err)
	}
	return true
}
