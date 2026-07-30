package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

func buildTLSConfig(config TLSConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}

	rootCertificates, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf(
			"load system certificate pool: %w",
			err,
		)
	}

	if rootCertificates == nil {
		rootCertificates = x509.NewCertPool()
	}

	if config.CAFile != "" {
		certificateAuthority, readErr := os.ReadFile(config.CAFile)
		if readErr != nil {
			return nil, fmt.Errorf(
				"read Kafka certificate authority: %w",
				readErr,
			)
		}

		if ok := rootCertificates.AppendCertsFromPEM(
			certificateAuthority,
		); !ok {
			return nil, fmt.Errorf(
				"parse Kafka certificate authority",
			)
		}
	}

	var certificates []tls.Certificate

	if config.CertFile != "" {
		certificate, loadErr := tls.LoadX509KeyPair(
			config.CertFile,
			config.KeyFile,
		)
		if loadErr != nil {
			return nil, fmt.Errorf(
				"load Kafka client certificate: %w",
				loadErr,
			)
		}

		certificates = append(certificates, certificate)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   config.ServerName,
		RootCAs:      rootCertificates,
		Certificates: certificates,
	}, nil
}

func buildSASLMechanism(
	config SASLConfig,
) (sasl.Mechanism, error) {
	switch config.Mechanism {
	case "":
		return nil, nil

	case "plain":
		return plain.Auth{
			User: config.Username,
			Pass: config.Password,
		}.AsMechanism(), nil

	case "scram-sha-256":
		return scram.Auth{
			User: config.Username,
			Pass: config.Password,
		}.AsSha256Mechanism(), nil

	case "scram-sha-512":
		return scram.Auth{
			User: config.Username,
			Pass: config.Password,
		}.AsSha512Mechanism(), nil

	default:
		return nil, fmt.Errorf(
			"unsupported Kafka SASL mechanism %q",
			config.Mechanism,
		)
	}
}
