package credentials

// Package credentials loads certificates for server

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	log "github.com/golang/glog"
)

var (
	caCert     = flag.String("ca-cert", "", "CA certificate file")
	serverCert = flag.String("server-cert", "", "Server certificate file")
	serverKey  = flag.String("server-key", "", "Server private key file")
	skipVerify = flag.Bool("skip-verify", false, "Skip TLS connection verfication")
	notls      = flag.Bool("no-tls", false, "Disable TLS (Transport Layer Security) for gRPC insecure mode")
)

// LoadCertificates loads certificates from file.
func LoadCertificates(cafile, certfile, keyfile string) ([]tls.Certificate, *x509.CertPool) {
	if cafile == "" || certfile == "" || keyfile == "" {
		log.Exit("ca-cert, sever-cert and server-key must be configured for TLS connection")
	}

	certificate, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		log.Exitf("could not load key pair: %s", err)
	}

	certPool := x509.NewCertPool()
	caFile, err := ioutil.ReadFile(cafile)
	if err != nil {
		log.Exitf("could not read CA certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(caFile); !ok {
		log.Exit("failed to append CA certificate")
	}

	return []tls.Certificate{certificate}, certPool
}

// ServerCredentials generates gRPC ServerOptions for existing credentials.
func ServerCredentials(cafile, certfile, keyfile string, skipVerifyTLS, noTLS bool) []grpc.ServerOption {
	if *notls || skipVerifyTLS {
		return []grpc.ServerOption{}
	}
	if cafile == "" {
		cafile = *caCert
	}
	if certfile == "" {
		certfile = *serverCert
	}
	if keyfile == "" {
		keyfile = *serverKey
	}

	certificates, certPool := LoadCertificates(cafile, certfile, keyfile)

	if *skipVerify || skipVerifyTLS {
		return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.VerifyClientCertIfGiven,
			Certificates: certificates,
			ClientCAs:    certPool,
		}))}
	}

	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certificates,
		ClientCAs:    certPool,
	}))}
}
