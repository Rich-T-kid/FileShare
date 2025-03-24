package main

import (
	"fmt"
	"log"
	"net"
	"sync"
)

var (
	connections = make(map[net.Conn]int)
	mu          sync.Mutex
)

// fan our patterns
func handleConnection(conn net.Conn, id int) {
	defer func() {
		mu.Lock()
		delete(connections, conn) // Remove connection when closed
		mu.Unlock()
		conn.Close()
	}()

	mu.Lock()
	connections[conn] = id // Add connection to the pool
	mu.Unlock()

	buffer := make([]byte, 2048)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Printf("Connection closed from %s\n", conn.RemoteAddr().String())
			return
		}
		fmt.Printf("Received %d bytes from %d: %s\n", n, id, string(buffer[:n]))

		// Broadcast the message to all connections
		broadcast(buffer[:n], conn, id)
	}

}
func broadcast(message []byte, sender net.Conn, id int) {
	fmt.Println("Current Connection pol", connections)
	for conn := range connections {
		//if conn != sender {
		prePend := []byte(fmt.Sprintf("userID: %d ", id))
		prePend = append(prePend, message...)
		_, err := conn.Write(prePend)
		if err != nil {
			fmt.Printf("Error writing to connection %s: %v\n", conn.RemoteAddr().String(), err)
		}
		//	}
	}
}

func main() {
	const connection = "localhost:8000"
	listener, err := net.Listen("tcp", connection)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Started server")
	defer listener.Close()
	var id int
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("Handlling new connection")
		id++
		go handleConnection(conn, id)

	}

}
