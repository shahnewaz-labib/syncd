package discover

import (
	"fmt"
	"net"
	"syncd/config"
	"time"
)

const (
	broadcastIP          = "255.255.255.255"
	port                 = 21027
	AnnouncementInterval = 5 * time.Second
)

func StartAnnouncementService(cfg *config.Config) {
	ticker := time.NewTicker(AnnouncementInterval)
	defer ticker.Stop()

	for range ticker.C {
		Announce(cfg.Name)
	}
}

func Announce(name string) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP(broadcastIP),
		Port: port,
	})

	if err != nil {
		fmt.Println("Dial error:", err)
		return
	}
	defer conn.Close()

	conn.SetWriteBuffer(1024)

	msg := fmt.Sprintf("name: %s\n", name)

	_, err = conn.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write error:", err)
	}
}
