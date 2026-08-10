package grpcx

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// WorkloadTLSFiles identifies the mounted workload certificate,
// private key and trust bundle.
type WorkloadTLSFiles struct {
	CertificateFile string
	PrivateKeyFile  string
	CAFile          string
}

// Validate verifies that every required file path is present.
func (files WorkloadTLSFiles) Validate() error {
	if strings.TrimSpace(files.CertificateFile) == "" {
		return fmt.Errorf(
			"workload TLS certificate file is required",
		)
	}

	if strings.TrimSpace(files.PrivateKeyFile) == "" {
		return fmt.Errorf(
			"workload TLS private key file is required",
		)
	}

	if strings.TrimSpace(files.CAFile) == "" {
		return fmt.Errorf(
			"workload TLS CA file is required",
		)
	}

	return nil
}

func loadWorkloadCertificate(
	files WorkloadTLSFiles,
) (
	tls.Certificate,
	*x509.CertPool,
	error,
) {
	if err := files.Validate(); err != nil {
		return tls.Certificate{}, nil, err
	}

	certificate, err := tls.LoadX509KeyPair(
		files.CertificateFile,
		files.PrivateKeyFile,
	)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf(
			"load workload TLS key pair: %w",
			err,
		)
	}

	caPEM, err := os.ReadFile(files.CAFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf(
			"read workload TLS CA: %w",
			err,
		)
	}

	certificatePool := x509.NewCertPool()

	if !certificatePool.AppendCertsFromPEM(
		caPEM,
	) {
		return tls.Certificate{}, nil, fmt.Errorf(
			"workload TLS CA contains no valid certificates",
		)
	}

	return certificate, certificatePool, nil
}

// LoadWorkloadServerTLS builds an mTLS server configuration.
//
// Every caller must present a certificate chained to the configured
// platform workload CA. Service-specific interceptors may additionally
// authorize the peer's SPIFFE URI SAN.
func LoadWorkloadServerTLS(
	files WorkloadTLSFiles,
) (*tls.Config, error) {
	certificate, certificatePool, err :=
		loadWorkloadCertificate(files)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,

		Certificates: []tls.Certificate{
			certificate,
		},

		ClientCAs:  certificatePool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

// LoadWorkloadClientTLS builds an mTLS client configuration.
func LoadWorkloadClientTLS(
	files WorkloadTLSFiles,
	serverName string,
) (*tls.Config, error) {
	serverName = strings.TrimSpace(serverName)

	if serverName == "" {
		return nil, fmt.Errorf(
			"gRPC TLS server name is required",
		)
	}

	certificate, certificatePool, err :=
		loadWorkloadCertificate(files)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,

		Certificates: []tls.Certificate{
			certificate,
		},

		RootCAs:    certificatePool,
		ServerName: serverName,
	}, nil
}
