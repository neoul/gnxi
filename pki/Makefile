ID?=1
.DEFAULT_GOAL := help
SHELL = /bin/bash

.EXPORT_ALL_VARIABLES:

CLIENT_NAME?=${USER}
SERVER_NAME?=grpc-notification-server

CERT_DATE?=36500

CA_FILE?=ca
CA_C?=KR
CA_L?=AY
CA_O?=gNMI target
CA_CN?=gNMI target's Self-signed CA

ADDR?=IP:127.0.0.1
# ADDR?=DNS:localhost
# ADDR?=IP:192.168.0.77
SERVER_CERT_FILE?=server
SERVER_CERT_C?=KR
SERVER_CERT_L?=AY
SERVER_CERT_O?=gNMI target
SERVER_CERT_CN?=gNMI target

CLIENT_CERT_FILE?=client
CLIENT_CERT_C?=KR
CLIENT_CERT_L?=AY
CLIENT_CERT_O?=gNMI client
CLIENT_CERT_CN?=gNMI Client

.PHONY: proto

all: cert-without-ca docker-build

generate-ca: ## Generate Self-signed CA
	openssl genrsa -out $(CA_FILE).key 2048
	openssl req -new -x509 -days $(CERT_DATE) -key $(CA_FILE).key -subj "/C=$(CA_C)/L=$(CA_L)/O=$(CA_O)/CN=$(CA_CN)" -out $(CA_FILE).crt

generate-cert-server: ## Generate server crt and key
	openssl req -newkey rsa:2048 -nodes -keyout $(SERVER_CERT_FILE).key -subj "/C=$(SERVER_CERT_C)/L=$(SERVER_CERT_L)/O=$(SERVER_CERT_O)/CN=$(SERVER_CERT_CN)" -out $(SERVER_CERT_FILE).csr
	# openssl x509 -req -extfile <(printf "subjectAltName=DNS:localhost") -days $(CERT_DATE) -in $(SERVER_CERT_FILE).csr -CA $(CA_FILE).crt -CAkey $(CA_FILE).key -CAcreateserial -out $(SERVER_CERT_FILE).crt
	# subjectAltName should be an IP address if it is self-signed CA for gRPC
	# openssl x509 -req -extfile <(printf "subjectAltName=$(ADDR)") -days $(CERT_DATE) -in $(SERVER_CERT_FILE).csr -CA $(CA_FILE).crt -CAkey $(CA_FILE).key -CAcreateserial -out $(SERVER_CERT_FILE).crt
	openssl x509 -req -extfile <(printf "subjectAltName=$(ADDR)") -days $(CERT_DATE) -in $(SERVER_CERT_FILE).csr -CA $(CA_FILE).crt -CAkey $(CA_FILE).key -CAcreateserial -out $(SERVER_CERT_FILE).crt

generate-cert-client: ## Generate client crt and key
	openssl req -newkey rsa:2048 -nodes -keyout $(CLIENT_CERT_FILE).key -subj "/C=$(CLIENT_CERT_C)/L=$(CLIENT_CERT_L)/O=$(CLIENT_CERT_O)/CN=$(CLIENT_CERT_CN)" -out $(CLIENT_CERT_FILE).csr
	openssl x509 -req -days $(CERT_DATE) -in $(CLIENT_CERT_FILE).csr -CA $(CA_FILE).crt -CAkey $(CA_FILE).key -CAcreateserial -out $(CLIENT_CERT_FILE).crt -sha256

clean: ## clean output
	rm -f *.crt
	rm -f *.csr
	rm -f *.key
	rm -f *.srl

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
