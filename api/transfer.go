package api

import (
	"net/http"

	"syncd/events"
	"syncd/transfer"
	"syncd/utils"

	"github.com/gin-gonic/gin"
)

func transferRequestHandler(c *gin.Context) {
	var request transfer.TransferRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store the pending request
	transfer.GetReceiver().AddPendingRequest(request)

	// Publish event to CLI
	events.PublishTransferRequest(events.TransferRequestPayload{
		TransferID: request.TransferID,
		SenderID:   request.SenderID,
		SenderName: request.SenderName,
		SenderIP:   request.SenderIP,
		FileName:   request.FileName,
		FileSize:   request.FileSize,
	})

	c.JSON(http.StatusAccepted, gin.H{"status": "pending"})
}

func transferResponseHandler(c *gin.Context) {
	var response transfer.TransferResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle the response (this is called on the sender side)
	transfer.GetSender().HandleResponse(response)

	// Publish event to CLI
	events.PublishTransferResponse(events.TransferResponsePayload{
		TransferID:   response.TransferID,
		Accepted:     response.Accepted,
		SavePath:     response.SavePath,
		ReceiverIP:   response.ReceiverIP,
		ReceiverPort: response.ReceiverPort,
	})

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

func transferStatusHandler(c *gin.Context) {
	transferID := c.Param("id")

	t := transfer.GetSender().GetTransfer(transferID)
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transfer not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transfer_id":       t.ID,
		"status":            t.Status,
		"bytes_transferred": t.BytesTransferred,
		"total_bytes":       t.Request.FileSize,
	})
}

func sendResponseToSender(senderIP string, response transfer.TransferResponse) error {
	// This will be called from CLI when user accepts/rejects
	return utils.PostJSON(senderIP, "/transfer/response", response)
}

// initiateTransferHandler allows CLI to instruct daemon to send a file
func initiateTransferHandler(c *gin.Context) {
	var req struct {
		TargetIP string `json:"target_ip"`
		FilePath string `json:"file_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceInfo, _ := utils.GetDeviceInfo()
	sender := transfer.GetSender()

	t, err := sender.InitiateTransfer(
		req.TargetIP,
		req.FilePath,
		deviceInfo.UniqueDeviceID,
		utils.GetUsername(),
		utils.GetLocalIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sender.SetFilePath(t.ID, req.FilePath)
	c.JSON(http.StatusAccepted, gin.H{"transfer_id": t.ID})
}

func RegisterTransferRoutes(r *gin.Engine) {
	r.POST("/transfer/request", transferRequestHandler)
	r.POST("/transfer/response", transferResponseHandler)
	r.GET("/transfer/status/:id", transferStatusHandler)
	r.POST("/transfer/initiate", initiateTransferHandler)
}
