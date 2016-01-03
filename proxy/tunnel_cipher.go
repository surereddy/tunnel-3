package proxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"io"
)

type (
	cipherMethods struct{}

	StreamCreator func(key, iv []byte, isEncrypt bool) (cipher.Stream, error)

	CipherMeta struct {
		keyLen int
		ivLen  int
		new    StreamCreator
	}

	Cipher struct {
		key []byte

		enc cipher.Stream
		dec cipher.Stream

		meta *CipherMeta
	}
)

func (cipherMethods) NewRc4Md5Stream(key, iv []byte, _ bool) (cipher.Stream, error) {
	m := md5.New()
	m.Write(key)
	m.Write(iv)
	c, err := rc4.NewCipher(m.Sum(nil))
	return c, err
}

func (cipherMethods) NewAESStream(key, iv []byte, isEncrypt bool) (cipher.Stream, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if isEncrypt {
		return cipher.NewCFBEncrypter(blk, iv), nil
	}
	return cipher.NewCFBDecrypter(blk, iv), nil
}

func NewCipherMeta(keyLen, ivLen int, newStream StreamCreator) *CipherMeta {
	return &CipherMeta{
		keyLen: keyLen,
		ivLen:  ivLen,
		new:    newStream,
	}
}

func (c *CipherMeta) NewZeroIv() []byte {
	return make([]byte, c.ivLen*2)
}

func (c *CipherMeta) NewIv() ([]byte, error) {
	iv := c.NewZeroIv()
	_, err := io.ReadFull(rand.Reader, iv)
	return iv, err
}

func (c *CipherMeta) NewStream(key, iv []byte, isEncrypt bool) (cipher.Stream, error) {
	key = c.genKey(key)
	iv = iv[len(iv)-c.ivLen:]
	return c.new(key, iv, isEncrypt)
}

func (c *CipherMeta) genKey(key []byte) []byte {
	const md5Len = 16

	var md5Sum = func(b []byte) []byte {
		m := md5.New()
		m.Write(b)
		return m.Sum(nil)
	}

	cnt := (c.keyLen-1)/md5Len + 1
	m := make([]byte, cnt*md5Len)
	copy(m, md5Sum(key))

	// Repeatedly call md5 until bytes generated is enough.
	// Each call to md5 uses data: prev md5 sum + key.
	d := make([]byte, md5Len+len(key))
	start := 0
	for i := 1; i < cnt; i++ {
		start += md5Len
		copy(d, m[start-md5Len:start])
		copy(d[md5Len:], key)
		copy(m[start:], md5Sum(d))
	}
	return m[:c.keyLen]
}

func NewCipher(key []byte, meta *CipherMeta) *Cipher {
	return &Cipher{
		key:  key,
		meta: meta,
	}
}

func (c *Cipher) NewZeroIv() []byte {
	return c.meta.NewZeroIv()
}

func (c *Cipher) Copy() *Cipher {
	return NewCipher(c.key, c.meta)
}

func (c *Cipher) InitEnc() ([]byte, error) {
	iv, err := c.meta.NewIv()
	if err == nil {
		c.enc, err = c.meta.NewStream(c.key, iv, true)
	}
	return iv, err
}

func (c *Cipher) IsEncInited() bool {
	return c.enc != nil
}

func (c *Cipher) Encrypt(dst, src []byte) {
	c.enc.XORKeyStream(dst, src)
}

func (c *Cipher) InitDec(iv []byte) error {
	var err error
	c.dec, err = c.meta.NewStream(c.key, iv, false)
	return err
}

func (c *Cipher) IsDecInited() bool {
	return c.dec != nil
}

func (c *Cipher) Decrypt(dst, src []byte) {
	c.dec.XORKeyStream(dst, src)
}
