package tracker

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"syncd/config"
)

type DeviceInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type DevicesResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Count   int          `json:"count"`
}

func devicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := GetDevices()
	deviceList := make([]DeviceInfo, 0, len(devices))

	for name, addr := range devices {
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			deviceList = append(deviceList, DeviceInfo{
				Name: name,
				IP:   udpAddr.IP.String(),
				Port: udpAddr.Port,
			})
		}
	}

	response := DevicesResponse{
		Devices: deviceList,
		Count:   len(deviceList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func StartTrackerAPI() {
	http.HandleFunc("/devices", devicesHandler)

	port := ":" + strconv.Itoa(config.TrackerAPIPort)
	fmt.Println("Tracker API started on port:", port)
	http.ListenAndServe(port, nil)
}
