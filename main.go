package main

import (
	"fmt"
	"time"

	"syncd/announcement"
	"syncd/utils"
)

const (
	PORT = 9999
)

func main() {
	fmt.Printf("My IP: %s\n", utils.GetLocalIP())
	go announcement.Listen()
	time.Sleep(100 * time.Millisecond)
	go announcement.Broadcast()
	select {} // Keep running
}
