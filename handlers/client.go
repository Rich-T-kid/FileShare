package handlers

import (
	"context"
	"fmt"
	"net"
)

type User struct {
}

func (u User) HandleConnection(ctx context.Context, conn net.Conn) {
	fmt.Println("Got to user Handler")
}
