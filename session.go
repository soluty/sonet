package sonet

import "net"

type session struct {
	id    uint64
	conn  net.Conn
	state interface{}
	app interface{}
}

func (s *session) ID() uint64 {
	return s.id
}

func (s *session) Conn() net.Conn {
	return s.conn
}

func (s *session) App() interface{} {
	return s.app
}

func (s *session) State() interface{} {
	return s.state
}

func (s *session) SetState(state interface{})  {
	 s.state=state
}

type Session interface {
	ID() uint64
	Conn() net.Conn
	State() interface{}
	SetState(interface{})
	App() interface{}
}
