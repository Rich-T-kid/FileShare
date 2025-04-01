package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.0:9999")
	if err != nil {
		log.Fatal(err)
	}
	f, _ := os.OpenFile("clientFile.txt", os.O_RDONLY, 0644)
	//data := make([]byte, 1024)
	//n, _ := f.Read(data)
	msg := fmt.Sprintf("recieve: \n FileName:%s", f.Name()+"incom")
	conn.Write([]byte(msg))
	rd := make([]byte, 1024*4)
	n, _ := conn.Read(rd)
	fmt.Printf("server wrote %v\n", string(rd[:n]))
	conn.Close()
	/*
	   var req = "recieve: \n FileName:clientFile.txt \n"
	   n, _ := conn.Write([]byte(req))
	   fmt.Printf("read %d bytes from connection", n)
	   nb := make([]byte, 1024*4)
	   n, _ = conn.Read(nb)
	   fmt.Printf("server response (%d bytes) -> %v\n", n, string(nb[:n]))
	*/
}
