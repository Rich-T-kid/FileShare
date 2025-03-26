package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// TODO: Should just be in global scope but for now its fine
var (
	logChan          = make(chan logMsg, 2)
	connections_map  = sync.Map{}
	totalConnections = make([]string, 1)
	mu               sync.Mutex
	id               int
	txt              = "The code above uses the os' package Open function to open the file, defers its Close function with the defer keyword, defines an empty lines slice, and uses the bufio's NewScanner function to read the file line-by-line while appending each line to the lines array in a text format using the Text function. Finally, it uses the printLastNLines function to get the last N lines of the lines array. N is any number of the user's choosing. In this case, it is 3, and the code uses a for loop to print each line with an horizontal line between each one."
)

type logMsg struct {
	data []byte
	name string
}

func registerClient(c net.Conn) string {
	name := c.RemoteAddr().String()
	totalConnections = append(totalConnections, name)
	connections_map.Store(id, name)
	return name
}
func unregisterClient(c net.Conn) {
	name := c.RemoteAddr().String()
	connections_map.Delete(name)
}
func readClientmsg(c net.Conn) string {
	// TODO: This wont always be enough to read in all the bytes
	reader := bufio.NewReader(c)
	line, err := reader.ReadString('\n') // Reads until newline
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		return ""
	}
	return strings.TrimSpace(line)
}

// Here we need to just register the conenction first
func handleConnection(conn net.Conn) {

	defer conn.Close()
	id := registerClient(conn)
	msg := fmt.Sprintf("Hello %v, you are now register in the file system \n", id)
	var b bytes.Buffer
	b.WriteString(msg)
	_, err := b.WriteTo(conn)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	// TODO move this stuff to a file so that it doesnt clutter up STOUT
	line := readClientmsg(conn)
	logmsg := fmt.Sprintf("client %s wrote: %s\n", id, line)
	logChan <- logMsg{
		data: []byte(logmsg),
		name: id,
	}
	time.Sleep(time.Second * 10)

}
func logConnections() {
	for {
		time.Sleep(time.Second * 15)
		connections_map.Range(func(key, value any) bool {
			fmt.Printf("Key: %v, Value: %v\n", key, value)
			return true // continue iteration
		})
	}
}
func logTotalConnections() {
	for {
		time.Sleep(time.Second * 30)
		for i := range totalConnections {
			fmt.Printf("i:%d conn:%s\n", i, totalConnections[i])
		}
	}
}
func handleLogs() {
	f, err := os.OpenFile("fs_logs.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for {
		logMessage := <-logChan
		fmt.Printf("%s sent %d bytes to be written to disk\n", logMessage.name, len(logMessage.data))
		f.Write(logMessage.data)
	}

}
func backgroundTask() {
	go logConnections()
	go logTotalConnections()
	go handleLogs()
}

func main() {
	var connectionStr = "localhost:8000"
	listener, err := net.Listen("tcp", connectionStr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	fmt.Printf("Started tcp Listener on %s\n", connectionStr)
	go backgroundTask()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		id++
		go handleConnection(conn)

	}

}
