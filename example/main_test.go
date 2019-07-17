package main

import (
	"fmt"
	"github.com/soluty/sonet"
	"testing"
	"time"
)

func TestDial(t *testing.T) {
	s := sonet.New(sonet.ServerConfig{Network: "test"})
	go s.Start("")
	time.Sleep(time.Millisecond)
	c, _ := sonet.Dial("test", "")
	c.Write([]byte("a"))
	b := make([]byte, 1)
	c.Read(b)
	fmt.Println(string(b))
	s.Stop()
	s.Wait()
}

func TestDialTcp(t *testing.T) {
	s := sonet.New()
	go s.Start(":8003")
	time.Sleep(100 * time.Millisecond)
	c, _ := sonet.Dial("tcp", "127.0.0.1:8003")
	c.Write([]byte("a"))
	b := make([]byte, 1)
	c.Read(b)
	fmt.Println(string(b))
	s.Stop()
	s.Wait()
}
