package domain

import "time"

// OutboxEvent is an event committed atomically with a domain mutation.
type OutboxEvent struct {
	ID         string
	Topic      string
	Key        string
	Envelope   []byte
	OccurredAt time.Time
}

// OutboxRecord is an unpublished database outbox row.
type OutboxRecord struct {
	OutboxID int64
	Event    OutboxEvent
	Attempts int
}
