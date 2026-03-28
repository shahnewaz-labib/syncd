package api

import (
	"net/http"
	"time"

	"syncd/announcement"
	"syncd/config"

	"github.com/gin-gonic/gin"
)

type DeviceResponse struct {
	DeviceID string    `json:"device_id"`
	Username string    `json:"username"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

func devicesHandler(c *gin.Context) {
	devices := announcement.GetOnlineDevices()
	response := make([]DeviceResponse, len(devices))

	for i, device := range devices {
		response[i] = DeviceResponse{
			DeviceID: device.DeviceID,
			Username: device.Username,
			IP:       device.IP,
			LastSeen: device.LastSeen,
		}
	}

	c.JSON(http.StatusOK, response)
}

func StartServer() {
	r := gin.Default()
	r.GET("/devices", devicesHandler)
	RegisterTransferRoutes(r)

	r.Run(":" + config.API_PORT)
}
