package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	for i := 1; i < 2; i++ {
		conn, err := net.Dial("tcp", "localhost:8000")
		if err != nil {
			log.Fatal(err)
		}
		p := make([]byte, 256)
		for {
			n, err := conn.Read(p)
			if err != nil {
				break
			}
			fmt.Printf("read %d bytes\n", n)
			fmt.Printf("from connection -> %s\n", p[:n])
			time.Sleep(time.Second * 5)
		}
		/*
			file, err := os.Open("clientFile.txt")
			if err != nil {
				fmt.Println("Error opening file:", err)
				return
			}
			defer file.Close()

			// Copy the file content to the connection
			_, err = io.Copy(conn, file)
			if err != nil {
				fmt.Println("Error copying data:", err)
				return
			}
		*/
		fmt.Println("File sent successfully")
	}
}
