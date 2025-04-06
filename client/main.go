package main

import (
	"fmt"
	"log"
	"net"
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
func main() {
	conn, err := net.Dial("tcp", "127.0.0.0:9999")
	if err != nil {
		log.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
	//	err = Register(conn)
	content := "This is just a lil thing i just cooked up"
	msg := fmt.Sprintf("store:\nFileName:RichardFille size:%d\n %v", len(content), []byte(content))
	fmt.Println("Writing", msg)
	conn.Write([]byte(msg))
	b := make([]byte, 212)
	n, _ := conn.Read(b)
	fmt.Printf("Server responded -> %v\n", string(b[:n]))

}
