package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

func main() {
	for i := 1; i < 2; i++ {
		conn, err := net.Dial("tcp", "localhost:8000")
		if err != nil {
			log.Fatal(err)
		}
		message := []byte("New messgge from the client!\n")
		_, err = conn.Write(message)
		if err != nil {
			log.Fatal("Error writing to server:", err)
		}
		defer conn.Close()
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

		fmt.Println("File sent successfully")
	}
}
