package domain

import "time"

// OutboxEvent is a pending outbox message.
type OutboxEvent struct {
	ID         string
	Topic      string
	Key        string
	Envelope   []byte
	OccurredAt time.Time
}

// OutboxRecord is a claimed outbox row.
type OutboxRecord struct {
	OutboxID int64
	Event    OutboxEvent
	Attempts int
}
