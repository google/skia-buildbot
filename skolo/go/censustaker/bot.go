package main

import "fmt"

type Bot struct {
	Hostname    string
	Port        int
	MACAddress  string
	IPV4Address string
}

func (b Bot) String() string {
	// This makes String() include the names of the fields in the struct.
	return fmt.Sprintf("%#v", b)
}
