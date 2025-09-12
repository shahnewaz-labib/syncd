package ui

import (
	"fmt"
	"net/http"
	"strconv"

	"syncd/announcement"
	"syncd/config"

	"github.com/gin-gonic/gin"
)

func devicesHTML(c *gin.Context) {
	devices := announcement.GetOnlineDevices()

	html := `<table>
		<thead>
			<tr>
				<th>Username</th>
				<th>Device ID</th>
				<th>IP Address</th>
				<th>Last Seen</th>
				<th>Status</th>
			</tr>
		</thead>
		<tbody>`

	if len(devices) == 0 {
		html += `<tr><td colspan="5">No active devices found</td></tr>`
	} else {
		for _, device := range devices {
			html += fmt.Sprintf(`
			<tr>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td><span class="status">Online</span></td>
			</tr>`,
				device.Username,
				device.DeviceID[:8]+"...", // Show only first 8 chars of device ID
				device.IP,
				device.LastSeen.Format("15:04:05"))
		}
	}

	html += `</tbody></table>`

	c.Data(http.StatusOK, "text/html", []byte(html))
}

func StartUIServer() {
	r := gin.Default()

	// Serve static files from ui directory
	r.Static("/static", "./ui")

	// Root redirects to index.html
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/index.html")
	})

	// HTMX endpoint for device table updates
	r.GET("/devices-table", devicesHTML)

	fmt.Printf("UI available at: http://localhost:%d\n", config.UI_PORT)
	r.Run(":" + strconv.Itoa(config.UI_PORT))
}
