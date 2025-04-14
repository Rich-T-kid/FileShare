package handlers

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// THis is for the storage instances that will be holding data (files)

/*
All messages are Prefixes with C:
(client,server) client -> server Response
(request to become register for file sharing,Acknologment)
c:/n State:Unregister\n -> OK
c:/n State:Register\n -> OK
(request polling to keep tcp connection,Acknologment)
c:/n State:Polling\n -> OK

This is where the actual mapping for the storage servers will be held
should move the  global variable to this file since its where it will be handled.
Before writing the logic , set up logging for external clients, and the server , storageMachines all at once.
*/
var (
	LoggerChan chan logMsg
)

const (
	Register   = "Register"
	Unregister = "Unregister"
	Polling    = "Polling"
)

type Client struct {
}

// repeat registrations have no affect
// must be registered to unregister
func (c Client) HandleConnection(ctx context.Context, conn net.Conn) {
	reader, ok := ctx.Value(connReader{}).(*bufio.Reader)
	if !ok {
		conn.Write([]byte(ERRinternalServer("Missing Buffer reader from middleware").Error()))
		return
	}
	rawBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println("read Bytes error", err)
		return
	}
	const prefix = "State:"
	line := strings.TrimSpace(string(rawBytes))
	if !strings.HasPrefix(line, prefix) {
		conn.Write([]byte(invalidRequest(line)))
		return
	}
	// throw away the bool because we are already checking for the prefix above
	command, _ := strings.CutPrefix(line, prefix)
	switch command {
	case Register:
		id := registerClient(conn)
		msg := fmt.Sprintf("OK!\n%s you are now registered to store files, @ %v\n", id, time.Now().Format("Monday, 02-Jan-06 15:04:05 MS"))
		conn.Write([]byte(msg))
		return
	case Unregister:
		exist := unregisterClient(conn)
		if !exist {
			conn.Write([]byte(invalidRequest(fmt.Sprintf("machine (%s) must already be registered to be unregistered\n", conn.RemoteAddr().String()))))
		}
		conn.Write([]byte("OK!\n You have succsufully unregistered"))
	case Polling:
		conn.Write([]byte("OK!\n"))

	default:
		conn.Write([]byte("Unsupported Command- No effect\n"))
		return
	}
}

func setexternalLog(sChan chan logMsg) {
	LoggerChan = sChan
}

// do not add a \n This will be handled by the logger before being written to disk
func writelog(msg []byte, name string) {
	LoggerChan <- logMsg{
		data: msg,
		name: name,
	}

}

func registerClient(c net.Conn) string {
	ipAndport := c.RemoteAddr().String()
	name := strings.Split(ipAndport, ":")
	totalConnections = append(totalConnections, name[0])
	connections_map.Store(name[0], id)
	return name[0]
}
func unregisterClient(c net.Conn) bool {
	//curTime := time.Now().Unix()
	//tempKey := strconv.FormatInt(curTime,10)
	ipAndport := c.RemoteAddr().String()
	name := strings.Split(ipAndport, ":")
	_, exist := connections_map.LoadAndDelete(name[0])

	return exist
}
