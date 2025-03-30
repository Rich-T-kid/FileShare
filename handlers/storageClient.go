package handlers

import (
	"context"
	"net"
)

// THis is for the storage instances that will be holding datap

/*
	All messages are Prefixes with C:
	(client,server) client -> server Response
	($shutdown_string,Acknologment)
	(request to become register for file sharing,Acknologment)
	(request polling to keep tcp connection,Acknologment)
*/
// This is for external clients that wish to have their files stored on the distributed server
type Client struct {
}

func (c Client) HandleConnection(ctx context.Context, conn net.Conn) {

}
