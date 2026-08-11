package domain

import "time"

// OutboxEvent is a transactional outbox entry produced inside a business
// transaction and published to Kafka by the relay.
type OutboxEvent struct {
	ID string

	Topic string
	Key   string

	Envelope []byte

	OccurredAt time.Time
}

// OutboxRecord is a claimed outbox row owned by the relay.
type OutboxRecord struct {
	OutboxID int64

	TenantID string

	EventID string

	Topic string
	Key   string

	Envelope []byte

	OccurredAt time.Time

	Attempts int
}
