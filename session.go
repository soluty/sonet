package sonet

import "net"

type Session struct {
	id    uint64
	conn  net.Conn
	State interface{}
}

func (s *Session) GetID() uint64 {
	return s.id
}

func (s *Session) GetConn() net.Conn {
	return s.conn
}
