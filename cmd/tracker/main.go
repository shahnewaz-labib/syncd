package tracker

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syncd/config"
	"syscall"
)

var (
	devices = make(map[string]net.Addr)
	mu      sync.RWMutex
)

func StartTracker() {
	addr := net.UDPAddr{
		Port: config.TrackerPort,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go StartTrackerAPI()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			msg := string(buffer[:n])
			if msg == "" {
				continue
			}

			mu.Lock()
			devices[msg] = remoteAddr
			mu.Unlock()
		}
	}()

	fmt.Println("Tracker started on port:", config.TrackerPort)

	// Wait for interrupt signal to gracefully shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("Shutting down tracker...")
}

func GetDevices() map[string]net.Addr {
	mu.RLock()
	defer mu.RUnlock()

	result := make(map[string]net.Addr)
	for k, v := range devices {
		result[k] = v
	}
	return result
}
