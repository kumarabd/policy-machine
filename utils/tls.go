package utils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

func LoadTLSCredentials(ca, cert, key string) (*tls.Config, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := os.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA's certificate, %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, errors.New("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		RootCAs:      certPool,
	}

	return config, nil
}
