package handlers

import (
	"context"
	"fmt"
	"net"

	"github.com/joho/godotenv"
)

// This is just for the interfaces and struct definitions

func init() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

type Handler interface {
	HandleConnection(context.Context, net.Conn)
}

func (s *Server) HandleConn(T string) (Handler, bool) {
	switch T {
	case ":c":
		fmt.Println("using client Handler")
		return &Client{}, false
	case "store:", "recieve:":
		fmt.Println("using Users Handler")
		return &User{}, false
	case s._shutDownString:
		return s, true
	}
	fmt.Println("using server handler")
	return s, false
}
