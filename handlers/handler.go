package handlers

import (
	"context"
	"fmt"
	"net"
	"strings"

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
		fmt.Println(T)
		if strings.HasPrefix(T, "store:") {
			return &User{operation: "store"}, false
		} else if strings.HasPrefix(T, "recieve:") {
			return &User{operation: "recieve"}, false
		} else {
			return &User{}, false
		}
	case s._shutDownString:
		return s, true
	}
	fmt.Println("using server handler")
	return s, false
}
