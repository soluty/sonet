package sonet

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var testServerMap sync.Map

func defaultHandler(conn net.Conn) {
	for {
		bs := make([]byte, 1)
		n, err := io.ReadFull(conn, bs)
		if ErrorNeedClose(err) {
			return
		}
		if n > 0 && err == nil {
			_, _ = conn.Write(bs[:n])
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type Server struct {
	l           net.Listener
	stopOnce    sync.Once
	stopFunc    context.CancelFunc
	ctx         context.Context // root context
	sigChan     chan os.Signal
	stopped     bool
	wg          sync.WaitGroup
	cfg         ServerConfig
	connCounter uint64
}

type ServerConfig struct {
	Network      string
	PanicHandler func(err interface{})
	Handler      func(conn net.Conn)
}

func New(cfgs ...ServerConfig) *Server {
	var cfg = mergeConfig(cfgs...)
	s := &Server{
		cfg: cfg,
	}

	s.ctx, s.stopFunc = context.WithCancel(context.Background())
	s.sigChan = make(chan os.Signal, 1)
	signal.Notify(s.sigChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	return s
}

func (s *Server) Start(address string) error {
	if s.cfg.Network == "test" {
		if _, loaded := testServerMap.LoadOrStore(address, s); loaded {
			return errors.New("server has started")
		}
		for {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				time.Sleep(time.Millisecond * 100)
			}
		}
	}
	l, err := net.Listen(s.cfg.Network, address)
	if err != nil {
		return err
	}
	s.l = l
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}
		c, err := accept(s.l)
		if err != nil {
			return err
		}
		s.Go(func() {
			s.handleNewConnection(c)
		})
	}
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

func (s *Server) handleNewConnection(conn net.Conn) {
	count := atomic.AddUint64(&s.connCounter, 1)
	defer func() {
		log.Println("handle connection", count, "over")
		err := recover()
		_ = conn.Close()
		if err != nil {
			s.handlePanic(err)
		}
	}()
	ctx, _ := context.WithCancel(s.ctx)
	s.Go(func() {
		<-ctx.Done()
		_ = conn.Close()
	})
	log.Println("default handler: add new connection", count)
	s.cfg.Handler(conn)
}

func (s *Server) handlePanic(err interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	s.cfg.PanicHandler(err)
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

func ErrorNeedClose(err error) bool {
	if err == nil {
		return false
	}
	if err == io.ErrClosedPipe || strings.Contains(err.Error(), "use of closed network connection") || err == io.EOF {
		return true
	}
	return false
}

func mergeConfig(cfgs ...ServerConfig) ServerConfig {
	var defaultCfg = ServerConfig{
		Network: "tcp",
		PanicHandler: func(err interface{}) {
			log.Println(err)
		},
		Handler: defaultHandler,
	}
	for _, cfg := range cfgs {
		if cfg.PanicHandler != nil {
			defaultCfg.PanicHandler = cfg.PanicHandler
		}
		if cfg.Handler != nil {
			defaultCfg.Handler = cfg.Handler
		}
		if cfg.Network != "" {
			defaultCfg.Network = cfg.Network
		}
	}
	return defaultCfg
}
