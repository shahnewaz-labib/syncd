package events

import (
	"sync"
)

type EventType string

const (
	TransferRequestReceived  EventType = "transfer_request_received"
	TransferResponseReceived EventType = "transfer_response_received"
	TransferProgress         EventType = "transfer_progress"
	TransferCompleted        EventType = "transfer_completed"
	TransferFailed           EventType = "transfer_failed"
)

type Event struct {
	Type    EventType
	Payload any
}

type TransferRequestPayload struct {
	TransferID string
	SenderID   string
	SenderName string
	SenderIP   string
	FileName   string
	FileSize   int64
}

type TransferResponsePayload struct {
	TransferID   string
	Accepted     bool
	SavePath     string
	ReceiverIP   string
	ReceiverPort int
}

type TransferProgressPayload struct {
	TransferID       string
	BytesTransferred int64
	TotalBytes       int64
}

var (
	eventChan     chan Event
	eventChanOnce sync.Once
)

func GetEventChannel() chan Event {
	eventChanOnce.Do(func() {
		eventChan = make(chan Event, 100)
	})
	return eventChan
}

func Publish(event Event) {
	select {
	case GetEventChannel() <- event:
	default:
		// Channel full, drop event
	}
}

func PublishTransferRequest(payload TransferRequestPayload) {
	Publish(Event{
		Type:    TransferRequestReceived,
		Payload: payload,
	})
}

func PublishTransferResponse(payload TransferResponsePayload) {
	Publish(Event{
		Type:    TransferResponseReceived,
		Payload: payload,
	})
}

func PublishProgress(transferID string, bytesTransferred, totalBytes int64) {
	Publish(Event{
		Type: TransferProgress,
		Payload: TransferProgressPayload{
			TransferID:       transferID,
			BytesTransferred: bytesTransferred,
			TotalBytes:       totalBytes,
		},
	})
}

func PublishCompleted(transferID string) {
	Publish(Event{
		Type:    TransferCompleted,
		Payload: transferID,
	})
}

func PublishFailed(transferID string, err error) {
	Publish(Event{
		Type: TransferFailed,
		Payload: map[string]any{
			"transfer_id": transferID,
			"error":       err.Error(),
		},
	})
}
