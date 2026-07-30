// Package kafka provides the shared Gereh Kafka event transport.
package kafka

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultProducerBatchBytes = int32(1_000_000)
	defaultMaxPollRecords     = 100
)

// Config defines Kafka client configuration.
type Config struct {
	Brokers                []string
	ClientID               string
	GroupID                string
	Topics                 []string
	DialTimeout            time.Duration
	RequestTimeoutOverhead time.Duration
	ProducerLinger         time.Duration
	ProducerBatchMaxBytes  int32
	RecordDeliveryTimeout  time.Duration
	MaxPollRecords         int
	SessionTimeout         time.Duration
	HeartbeatInterval      time.Duration
	TLS                    TLSConfig
	SASL                   SASLConfig
}

// TLSConfig defines Kafka TLS configuration.
type TLSConfig struct {
	Enabled    bool
	ServerName string
	CAFile     string
	CertFile   string
	KeyFile    string
}

// SASLConfig defines Kafka SASL authentication.
type SASLConfig struct {
	Mechanism string
	Username  string
	Password  string
}

// DefaultConfig returns production-oriented client defaults.
func DefaultConfig(clientID string) Config {
	return Config{
		ClientID:               clientID,
		DialTimeout:            10 * time.Second,
		RequestTimeoutOverhead: 10 * time.Second,
		ProducerLinger:         5 * time.Millisecond,
		ProducerBatchMaxBytes:  defaultProducerBatchBytes,
		RecordDeliveryTimeout:  30 * time.Second,
		MaxPollRecords:         defaultMaxPollRecords,
		SessionTimeout:         45 * time.Second,
		HeartbeatInterval:      3 * time.Second,
	}
}

// ConfigFromEnv creates Kafka configuration from environment variables.
func ConfigFromEnv(clientID string) (Config, error) {
	config := DefaultConfig(clientID)

	config.Brokers = splitCommaSeparated(
		os.Getenv("KAFKA_BROKERS"),
	)

	if value := strings.TrimSpace(os.Getenv("KAFKA_CLIENT_ID")); value != "" {
		config.ClientID = value
	}

	config.GroupID = strings.TrimSpace(
		os.Getenv("KAFKA_GROUP_ID"),
	)

	config.Topics = splitCommaSeparated(
		os.Getenv("KAFKA_TOPICS"),
	)

	var err error

	config.DialTimeout, err = durationEnvironment(
		"KAFKA_DIAL_TIMEOUT",
		config.DialTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	config.RequestTimeoutOverhead, err = durationEnvironment(
		"KAFKA_REQUEST_TIMEOUT_OVERHEAD",
		config.RequestTimeoutOverhead,
	)
	if err != nil {
		return Config{}, err
	}

	config.ProducerLinger, err = durationEnvironment(
		"KAFKA_PRODUCER_LINGER",
		config.ProducerLinger,
	)
	if err != nil {
		return Config{}, err
	}

	config.RecordDeliveryTimeout, err = durationEnvironment(
		"KAFKA_RECORD_DELIVERY_TIMEOUT",
		config.RecordDeliveryTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	config.SessionTimeout, err = durationEnvironment(
		"KAFKA_SESSION_TIMEOUT",
		config.SessionTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	config.HeartbeatInterval, err = durationEnvironment(
		"KAFKA_HEARTBEAT_INTERVAL",
		config.HeartbeatInterval,
	)
	if err != nil {
		return Config{}, err
	}

	config.MaxPollRecords, err = integerEnvironment(
		"KAFKA_MAX_POLL_RECORDS",
		config.MaxPollRecords,
	)
	if err != nil {
		return Config{}, err
	}

	producerBatchBytes, err := integerEnvironment(
		"KAFKA_PRODUCER_BATCH_MAX_BYTES",
		int(config.ProducerBatchMaxBytes),
	)
	if err != nil {
		return Config{}, err
	}

	config.ProducerBatchMaxBytes = int32(producerBatchBytes)

	config.TLS.Enabled, err = booleanEnvironment(
		"KAFKA_TLS_ENABLED",
		false,
	)
	if err != nil {
		return Config{}, err
	}

	config.TLS.ServerName = strings.TrimSpace(
		os.Getenv("KAFKA_TLS_SERVER_NAME"),
	)
	config.TLS.CAFile = strings.TrimSpace(
		os.Getenv("KAFKA_TLS_CA_FILE"),
	)
	config.TLS.CertFile = strings.TrimSpace(
		os.Getenv("KAFKA_TLS_CERT_FILE"),
	)
	config.TLS.KeyFile = strings.TrimSpace(
		os.Getenv("KAFKA_TLS_KEY_FILE"),
	)

	config.SASL.Mechanism = strings.ToLower(
		strings.TrimSpace(
			os.Getenv("KAFKA_SASL_MECHANISM"),
		),
	)
	config.SASL.Username = os.Getenv("KAFKA_SASL_USERNAME")
	config.SASL.Password = os.Getenv("KAFKA_SASL_PASSWORD")

	if err := config.ValidateCommon(); err != nil {
		return Config{}, err
	}

	return config, nil
}

// ValidateCommon validates configuration shared by producers and consumers.
func (config Config) ValidateCommon() error {
	if len(config.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}

	if strings.TrimSpace(config.ClientID) == "" {
		return fmt.Errorf("kafka client ID is required")
	}

	if config.DialTimeout <= 0 {
		return fmt.Errorf("kafka dial timeout must be greater than zero")
	}

	if config.RequestTimeoutOverhead <= 0 {
		return fmt.Errorf(
			"kafka request timeout overhead must be greater than zero",
		)
	}

	if config.ProducerBatchMaxBytes < 512 {
		return fmt.Errorf(
			"kafka producer batch maximum must be at least 512 bytes",
		)
	}

	if config.MaxPollRecords <= 0 {
		return fmt.Errorf(
			"kafka maximum poll records must be greater than zero",
		)
	}

	if (config.TLS.CertFile == "") != (config.TLS.KeyFile == "") {
		return fmt.Errorf(
			"kafka TLS certificate and key must be configured together",
		)
	}

	switch config.SASL.Mechanism {
	case "", "plain", "scram-sha-256", "scram-sha-512":
	default:
		return fmt.Errorf(
			"unsupported Kafka SASL mechanism %q",
			config.SASL.Mechanism,
		)
	}

	if config.SASL.Mechanism != "" &&
		(config.SASL.Username == "" || config.SASL.Password == "") {
		return fmt.Errorf(
			"kafka SASL username and password are required",
		)
	}

	return nil
}

// ValidateConsumer validates consumer-specific configuration.
func (config Config) ValidateConsumer() error {
	if err := config.ValidateCommon(); err != nil {
		return err
	}

	if strings.TrimSpace(config.GroupID) == "" {
		return fmt.Errorf("kafka consumer group ID is required")
	}

	if len(config.Topics) == 0 {
		return fmt.Errorf("at least one Kafka consumer topic is required")
	}

	if config.SessionTimeout <= 0 {
		return fmt.Errorf(
			"kafka session timeout must be greater than zero",
		)
	}

	if config.HeartbeatInterval <= 0 {
		return fmt.Errorf(
			"kafka heartbeat interval must be greater than zero",
		)
	}

	if config.HeartbeatInterval >= config.SessionTimeout {
		return fmt.Errorf(
			"kafka heartbeat interval must be shorter than session timeout",
		)
	}

	return nil
}

func splitCommaSeparated(value string) []string {
	var values []string

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)

		if item != "" {
			values = append(values, item)
		}
	}

	return values
}

func durationEnvironment(
	name string,
	fallback time.Duration,
) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return duration, nil
}

func integerEnvironment(
	name string,
	fallback int,
) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	integer, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return integer, nil
}

func booleanEnvironment(
	name string,
	fallback bool,
) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	boolean, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}

	return boolean, nil
}
