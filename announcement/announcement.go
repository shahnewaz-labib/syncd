package announcement

import (
	"fmt"
	"net"
	"time"

	"syncd/utils"
)

type Announcement struct {
	Message string
}

func New() Announcement {
	return Announcement{
		Message: "SYNCD_DISCOVER",
	}
}

func (a Announcement) ToBytes() []byte {
	return []byte(a.Message)
}

func (a Announcement) String() string {
	return a.Message
}

const PORT = 9999

func Broadcast() {
	conn, err := net.Dial("udp", "255.255.255.255:"+fmt.Sprint(PORT))
	if err != nil {
		fmt.Printf("Broadcast error: %v\n", err)
		return
	}
	defer conn.Close()

	announcement := New()
	for {
		conn.Write(announcement.ToBytes())
		time.Sleep(2 * time.Second)
	}
}

func Listen() {
	addr, err := net.ResolveUDPAddr("udp", ":"+fmt.Sprint(PORT))
	if err != nil {
		fmt.Printf("Address error: %v\n", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Listen error: %v\n", err)
		return
	}
	defer conn.Close()

	myIP := utils.GetLocalIP()
	expectedAnnouncement := New()
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			continue
		}
		if string(buf[:n]) == expectedAnnouncement.Message {
			if clientAddr.IP.String() == myIP {
				fmt.Printf("Player found: %s (self)\n", clientAddr.IP)
			} else {
				fmt.Printf("Player found: %s\n", clientAddr.IP)
			}
		}
	}
}
