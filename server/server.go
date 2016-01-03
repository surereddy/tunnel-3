package server

import "strings"

const (
	MODE_LOCAL  = "local"
	MODE_REMOTE = "remote"
)

func isConnClosed(err error) bool {
	const ERR_CONN_CLOSED = "use of closed network connection"
	return strings.Contains(err.Error(), ERR_CONN_CLOSED)
}
