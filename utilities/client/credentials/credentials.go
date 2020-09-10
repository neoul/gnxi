/* Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package credentials loads certificates and validates user credentials.
package credentials

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	log "github.com/golang/glog"
)

var (
	ca             = flag.String("ca", "", "CA certificate file.")
	cert           = flag.String("cert", "", "Certificate file.")
	key            = flag.String("key", "", "Private key file.")
	insecure       = flag.Bool("insecure", false, "Skip TLS validation.")
	notls          = flag.Bool("notls", false, "Disable TLS validation. If true, no need to specify TLS related options.")
	authorizedUser = userCredentials{}
	usernameKey    = "username"
	passwordKey    = "password"
)

func init() {
	flag.StringVar(&authorizedUser.username, "username", "", "If specified, uses username/password credentials.")
	flag.StringVar(&authorizedUser.password, "password", "", "The password matching the provided username.")
}

type userCredentials struct {
	username string
	password string
}

func (a *userCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		usernameKey: a.username,
		passwordKey: a.password,
	}, nil
}

func (a *userCredentials) RequireTransportSecurity() bool {
	return true
}

// LoadCertificates loads certificates from file.
func LoadCertificates() ([]tls.Certificate, *x509.CertPool) {
	if *ca == "" || *cert == "" || *key == "" {
		log.Exit("-ca -cert and -key must be set with file locations")
	}

	certificate, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		log.Exitf("could not load client key pair: %s", err)
	}

	certPool := x509.NewCertPool()
	caFile, err := ioutil.ReadFile(*ca)
	if err != nil {
		log.Exitf("could not read CA certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(caFile); !ok {
		log.Exit("failed to append CA certificate")
	}

	return []tls.Certificate{certificate}, certPool
}

// ClientCredentials generates gRPC DialOptions for existing credentials.
func ClientCredentials(server string) []grpc.DialOption {

	opts := []grpc.DialOption{}

	if *notls {
		opts = append(opts, grpc.WithInsecure())
	} else {
		tlsConfig := &tls.Config{}
		if *insecure {
			tlsConfig.InsecureSkipVerify = true
		} else {
			certificates, certPool := LoadCertificates()
			tlsConfig.ServerName = server
			tlsConfig.Certificates = certificates
			tlsConfig.RootCAs = certPool
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}

	if authorizedUser.username != "" {
		return append(opts, grpc.WithPerRPCCredentials(&authorizedUser))
	}
	return opts
}
