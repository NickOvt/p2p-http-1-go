package main

import (
	"net"
)

type Client struct {
	remoteAddr string
	conn       net.Conn
}
