package main

import (
	"fmt"
	"net"
)

type Client struct {
	remoteAddr string
	conn       net.Conn
}

func connectToClient(addr string) *Client {
	clientConn, err := net.Dial("tcp", addr)

	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("Connected to client: %s\n", clientConn.RemoteAddr().String())

	go readLoop(clientConn, "CLIENT")

	return &Client{remoteAddr: clientConn.RemoteAddr().String(), conn: clientConn}
}
