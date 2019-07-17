package main

import (
	"github.com/soluty/sonet"
)

func main() {
	s := sonet.New()
	go s.Start(":8003")
	s.Wait()
}
