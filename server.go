package sonet

import (
	"context"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	l net.Listener
	stopOnce sync.Once
	stopFunc context.CancelFunc
	ctx context.Context // root context
	sigChan chan os.Signal
	stopped bool
	wg sync.WaitGroup
}

type ServerConfig struct {
	Network string
	Address string
}

func New(cfgs ...ServerConfig) *Server {
	s := &Server{}

	s.ctx, s.stopFunc = context.WithCancel(context.Background())
	s.sigChan = make(chan os.Signal,1)
	signal.Notify(s.sigChan, os.Interrupt,os.Kill,syscall.SIGTERM,syscall.SIGQUIT)
	return s
}

func (s *Server) Start() error {

}

func (s *Server) Wait() {
	<-s.sigChan
	s.stopped = true
	s.stopFunc()
	s.wg.Wait()
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.sigChan <- os.Interrupt
	})
}

func (s *Server) Go(f func()) {
	s.wg.Add(1)
	go func() {
		f()
		s.wg.Done()
	}()
}

func accept(listener net.Listener) (net.Conn, error) {
	var tempDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return nil, err
		}
		return conn, nil
	}
}

func ErrorNeedClose(err error) bool{
	if err == nil {
		return false
	}
	if err == io.ErrClosedPipe || strings.Contains(err.Error(), "use of closed network connection") || err == io.EOF {
		return true
	}
	return false
}