package handlers

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/joho/godotenv"
)

// This is just for the interfaces and struct definitions
var (
	ERRinternalServer = func(e string) error {
		return fmt.Errorf("internal server error has occured,context: %s", e)
	}
	invalidRequest = func(str string) string {
		return fmt.Sprintf("Malformed request sent, failed Command: %s\n", str)
	}
)

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
			// this is impossible Remove later
			return &User{}, false
		}
	case s._shutDownString:
		return s, true
	}
	fmt.Println("using server handler")
	return s, false
}
