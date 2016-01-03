package proxy

import (
	"io"
	"net"
)

type (
	Conn struct {
		cipher *Cipher
		net.Conn
	}
)

func NewConn(conn net.Conn, cipher *Cipher) net.Conn {
	return &Conn{
		Conn:   conn,
		cipher: cipher,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	if !c.cipher.IsDecInited() {
		iv := c.cipher.NewZeroIv()
		_, err := io.ReadFull(c.Conn, iv)
		if err != nil {
			return 0, err
		}

		err = c.cipher.InitDec(iv)
		if err != nil {
			return 0, err
		}
	}
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.cipher.Decrypt(b[:n], b[:n])
	}
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	var ivLen int
	if !c.cipher.IsEncInited() {
		iv, err := c.cipher.InitEnc()
		if err != nil {
			return 0, err
		}

		ivLen = len(iv)
		encData := make([]byte, len(b)+ivLen)
		copy(encData, iv)
		c.cipher.Encrypt(encData[ivLen:], b)
		b = encData
	} else {
		c.cipher.Encrypt(b, b)
	}
	n, err := c.Conn.Write(b)
	if n >= ivLen {
		n -= ivLen
	}
	return n, err
}
