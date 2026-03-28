package transfer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

type TransferRequest struct {
	TransferID string `json:"transfer_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	SenderIP   string `json:"sender_ip"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	Checksum   string `json:"checksum"`
}

type TransferResponse struct {
	TransferID   string `json:"transfer_id"`
	Accepted     bool   `json:"accepted"`
	SavePath     string `json:"save_path,omitempty"`
	ReceiverIP   string `json:"receiver_ip"`
	ReceiverPort int    `json:"receiver_port"`
	Error        string `json:"error,omitempty"`
}

type TransferStatus string

const (
	StatusPending    TransferStatus = "pending"
	StatusInProgress TransferStatus = "in_progress"
	StatusCompleted  TransferStatus = "completed"
	StatusFailed     TransferStatus = "failed"
	StatusRejected   TransferStatus = "rejected"
)

type Transfer struct {
	ID               string
	Request          TransferRequest
	Response         *TransferResponse
	Status           TransferStatus
	BytesTransferred int64
	StartTime        time.Time
	EndTime          time.Time
	Error            error
}

func GenerateTransferID() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))))[:16]
}

func CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
