package server

type Signal chan struct{}

func NewSignal() Signal {
	return make(chan struct{})
}

func (s Signal) Close() {
	close(s)
}

func (s Signal) Wait() {
	<-s
}
