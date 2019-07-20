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

func defaultHandler(s Session) {
	conn := s.Conn()
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
	stopped     bool
	ctx         context.Context // root context
	sigChan     chan os.Signal
	wg          sync.WaitGroup
	cfg         ServerConfig
	connCounter uint64
}

type ServerConfig struct {
	Network       string
	PanicHandler  func(session Session, err interface{})
	Handler       func(session Session)
	RemoveHandler func(session Session)
	App           interface{}
}

func New(configs ...ServerConfig) *Server {
	var cfg = mergeConfig(configs...)
	s := &Server{
		cfg: cfg,
	}
	s.ctx, s.stopFunc = context.WithCancel(context.Background())
	s.sigChan = make(chan os.Signal, 1)
	signal.Notify(s.sigChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	return s
}

func (s *Server) SetListener(l net.Listener) {
	s.l = l
}

func (s *Server) Start(address string) {
	if s.cfg.Network == "test" {
		if _, loaded := testServerMap.LoadOrStore(address, s); loaded {
			panic("server has started")
		}
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				time.Sleep(time.Millisecond * 100)
			}
		}
	}
	if s.l == nil {
		l, err := net.Listen(s.cfg.Network, address)
		if err != nil {
			panic(err.Error())
			return
		}
		s.l = l
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		c, err := s.accept()
		if err != nil {
			return
		}
		s.Go(func(context.Context) {
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

func (s *Server) Go(f func(ctx context.Context)) {
	s.wg.Add(1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println(err)
			}
			s.wg.Done()
		}()
		f(s.ctx)
	}()
}

func (s *Server) handleNewConnection(conn net.Conn) {
	session := s.newSession(conn)
	defer func() {
		log.Println("handle connection", session.ID(), "over")
		err := recover()
		_ = conn.Close()
		if err != nil {
			s.handlePanic(session, err)
		}
		s.cfg.RemoveHandler(session)
	}()
	ctx, _ := context.WithCancel(s.ctx)
	s.Go(func(context.Context) {
		<-ctx.Done()
		_ = conn.Close()
	})
	log.Println("handle new connection", session.ID())
	s.cfg.Handler(session)
}

func (s *Server) handlePanic(session Session, err interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	s.cfg.PanicHandler(session, err)
}

func (s *Server) newSession(conn net.Conn) Session {
	count := atomic.AddUint64(&s.connCounter, 1)
	return &session{
		id:   count,
		conn: conn,
		app:  s.cfg.App,
	}
}

func (s *Server) accept() (net.Conn, error) {
	var tempDelay time.Duration
	for {
		if s.stopped {
			return nil, errors.New("server stopped")
		}
		conn, err := s.l.Accept()
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

func mergeConfig(configs ...ServerConfig) ServerConfig {
	var defaultCfg = ServerConfig{
		Network: "tcp",
		PanicHandler: func(s Session, err interface{}) {
			log.Println(err)
		},
		Handler: defaultHandler,
		RemoveHandler: func(s Session) {
		},
	}
	for _, cfg := range configs {
		if cfg.PanicHandler != nil {
			defaultCfg.PanicHandler = cfg.PanicHandler
		}
		if cfg.Handler != nil {
			defaultCfg.Handler = cfg.Handler
		}
		if cfg.RemoveHandler != nil {
			defaultCfg.RemoveHandler = cfg.RemoveHandler
		}
		if cfg.Network != "" {
			defaultCfg.Network = cfg.Network
		}
		if cfg.App != nil {
			defaultCfg.App = cfg.App
		}
	}
	return defaultCfg
}
