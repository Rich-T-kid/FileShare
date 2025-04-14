package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func Register(c net.Conn) error {
	msg := ":c\n State:Register\n"
	c.Write([]byte(msg))

	buff := make([]byte, 512)
	n, err := c.Read(buff)
	if err != nil {
		return err
	}
	fmt.Printf("read %d bytes from connection. Server: %v\n", n, string(buff[:n]))
	return nil
}
func Unregister(c net.Conn) error {
	msg := ":c\n State:Unregister\n"
	c.Write([]byte(msg))

	buff := make([]byte, 512)
	n, err := c.Read(buff)
	if err != nil {
		return err
	}
	fmt.Printf("read %d bytes from connection. Server: %v\n", n, string(buff[:n]))
	return nil

}
func polling(c net.Conn) error {
	msg := ":c\n State:Polling\n"
	var count int
	buff := make([]byte, 512)
	c.Write([]byte(msg))
	n, err := c.Read(buff)
	if err != nil {
		return fmt.Errorf("failed on iteration %d with error: %w", count, err)
	}
	fmt.Printf("read %d bytes from connection. Server: %v\n", n, string(buff[:n]))
	return nil
}
func heartBeat() {
	server, err := net.Listen("tcp", ":9998")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			buff := make([]byte, 512)
			n, err := c.Read(buff)
			if err != nil {
				log.Fatal(err)
			}
			input := strings.TrimSpace(string(buff[:n]))
			if input == "ping" {
				c.Write([]byte("pong"))
				return
			} else {

				c.Write([]byte("not ping"))
			}
		}(conn)
	}
}
func save() {
	listener, err := net.Listen("tcp", ":9997")
	if err != nil {
		log.Fatal("Failed to start file server:", err)
	}
	fmt.Println("File server listening on port 9997")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()

			reader := bufio.NewReader(c)

			// Step 1: read file ID (ends with newline)
			fileID, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read file ID:", err)
				return
			}
			fileID = strings.TrimSpace(fileID)

			// Step 2: create file to store data
			f, err := os.Create(fileID + "_storage")
			if err != nil {
				log.Println("Failed to create file:", err)
				return
			}
			defer f.Close()

			// Step 3: copy the rest of the connection into the file
			written, err := io.Copy(f, reader)
			if err != nil {
				log.Println("Failed to write file contents:", err)
				return
			}

			fmt.Printf("Saved %d bytes to %s\n", written, fileID)
			c.Write([]byte("OK"))
		}(conn)
	}
}
func serveChunk() {
	listener, err := net.Listen("tcp", ":9996")
	if err != nil {
		log.Fatalf("Failed to start chunk server: %v", err)
	}
	fmt.Println("Storage server ready on port 9996")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection error:", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			reader := bufio.NewReader(c)

			// Read "<fileID>:<idx>\n" (but ignore idx)
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read request:", err)
				return
			}

			fileID := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
			fmt.Printf("Request received for file: %s\n", fileID)

			// Try to open the file
			file, err := os.Open(fileID)
			if err != nil {
				log.Printf("Failed to open file %s: %v\n", fileID, err)
				c.Write([]byte("ERROR: file not found\n"))
				return
			}
			defer file.Close()

			// Stream file back to requester
			n, err := io.Copy(c, file)
			if err != nil {
				log.Printf("Error sending file %s: %v\n", fileID, err)
			} else {
				fmt.Printf("Served %d bytes for file %s\n", n, fileID)
			}
		}(conn)
	}
}

// Note to self. Always write the data exactly as you expect to recieve it
// So if u want to store it as text and receive it as text dont send it as bytes and then expect text.
// Sprintf it into a string and send it as a string, golang will handle turning it into bytes
func main() {
	go heartBeat()
	go save()
	go serveChunk()
	wg := &sync.WaitGroup{}
	wg.Add(1) // never exits
	conn, err := net.Dial("tcp", "127.0.0.0:9999")
	if err != nil {
		log.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
	//err = Register(conn)
	f, _ := os.OpenFile("input.txt", os.O_RDWR|os.O_APPEND, 0666)
	defer f.Close()
	buff := make([]byte, 4096*3)
	n, _ := f.Read(buff)
	n, _ = conn.Write([]byte(fmt.Sprintf("store: \nFileName:%s size:%d\n %v", "Luffy-Uzumaki.txt", n, string(buff[:n]))))
	fmt.Printf("wrote %d bytes to server\n", n)
	resp := make([]byte, 512)
	n, _ = conn.Read(resp)
	fmt.Printf("read %d bytes from connection\n Server: %v\n", n, string(resp[:n]))
	wg.Wait()
	//fmt.Printf("read %d bytes from file\n %v", n, string(buff[:n]))
	//name := "Luffy Uzumaki"
	/*FileID := "403c9702b3219f8c"
	conn.Write([]byte(fmt.Sprintf("recieve: \nFileName:%s\n", FileID)))
	resp := make([]byte, 4054)
	outpulFile, err := os.OpenFile("output.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer outpulFile.Close()
	n, err := conn.Read(resp)
	if err != nil {
		log.Fatal(err)
	}
	outpulFile.Write(resp[:n])
	raw := resp[:n]
	fmt.Printf("read %d bytes from connection\n Server: %v\n", n, string(raw))
	*/
}
