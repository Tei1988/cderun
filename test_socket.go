package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	socketPath := "/tmp/test.sock"
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Printf("Listen error: %v\n", err)
		return
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		fmt.Println("Server accepted connection")
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Server read error: %v\n", err)
			return
		}
		fmt.Printf("Server read %d bytes: %s\n", n, string(buf[:n]))

		// Wait a bit before echoing
		time.Sleep(100 * time.Millisecond)
		_, err = conn.Write([]byte("echo: " + string(buf[:n])))
		if err != nil {
			fmt.Printf("Server write error: %v\n", err)
		}
		fmt.Println("Server finished writing")
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Printf("Dial error: %v\n", err)
		return
	}
	defer conn.Close()

	unixConn := conn.(*net.UnixConn)

	fmt.Println("Client sending data")
	_, _ = unixConn.Write([]byte("hello"))

	fmt.Println("Client calling CloseWrite")
	err = unixConn.CloseWrite()
	if err != nil {
		fmt.Printf("Client CloseWrite error: %v\n", err)
	}

	fmt.Println("Client reading response")
	buf := make([]byte, 1024)
	n, err := unixConn.Read(buf)
	if err != nil {
		fmt.Printf("Client read error: %v\n", err)
	} else {
		fmt.Printf("Client read %d bytes: %s\n", n, string(buf[:n]))
	}
}
