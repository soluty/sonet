package sonet

import (
	"context"
	"errors"
	"net"
)

var dialer Dialer

func SetDialer(d Dialer) {
	dialer = d
}

type testDialerFunc func(address string) (net.Conn, error)

func (f testDialerFunc) Dial(network, address string) (net.Conn, error) {
	return f(address)
}

var testDialer = func(address string) (net.Conn, error) {
	c, s := net.Pipe()
	if server, ok := testServerMap.Load(address); ok {
		ss := server.(*Server)
		ss.Go(func(ctx context.Context) {
			ss.handleNewConnection(s)
		})
	} else {
		return nil, errors.New("cant connect address " + address)
	}
	return c, nil
}

type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

func Dial(network, address string) (net.Conn, error) {
	if network == "test" {
		SetDialer(testDialerFunc(testDialer))
	}
	if dialer == nil {
		return net.Dial(network, address)
	} else {
		return dialer.Dial(network, address)
	}
}

// MustDial简化做单元测试的时候写
func MustDial(address string) net.Conn {
	conn, err := Dial("test", address)
	if err != nil {
		panic(err)
	}
	return conn
}
