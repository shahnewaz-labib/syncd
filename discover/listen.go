package discover

import (
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	deviceList      = make(map[string]time.Time)
	deviceListMutex sync.Mutex
)

const (
	AnnouncementCacheTTL = 10 * time.Second
)

func Listen() {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer conn.Close()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			deviceListMutex.Lock()
			now := time.Now()
			for name, lastSeen := range deviceList {
				if now.Sub(lastSeen) > AnnouncementCacheTTL {
					delete(deviceList, name)
				}
			}
			deviceListMutex.Unlock()
		}
	}()

	for {
		buffer := make([]byte, 1024)
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		msg := string(buffer[:n])
		fmt.Printf("Received message from %s: %s\n", remoteAddr, msg)

		if msg == "" {
			continue
		}

		var name string
		fmt.Sscanf(msg, "name: %s", &name)
		if name == "" {
			fmt.Println("No name found in message")
			continue
		}

		deviceListMutex.Lock()
		if _, exists := deviceList[name]; !exists {
			deviceList[name] = time.Now()
		}
		fmt.Println("Current devices:")
		for device := range deviceList {
			fmt.Println("-", device)
		}
		deviceListMutex.Unlock()
	}
}
