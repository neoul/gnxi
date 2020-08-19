// Package utilities implements utilities for gnxi.
package utilities

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// GetMetadata checks for valid credentials in the context Metadata.
func GetMetadata(ctx context.Context) (map[string]string, bool) {
	m := map[string]string{}
	headers, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return m, false
	}
	for k, v := range headers {
		k := strings.Trim(k, ":")
		m[k] = v[0]
	}
	p, ok := peer.FromContext(ctx)
	if ok {
		m["protocol"] = p.Addr.Network()
		m["peer"] = p.Addr.String()
		index := strings.LastIndex(p.Addr.String(), ":")
		m["peer-address"] = p.Addr.String()[:index]
		m["peer-port"] = p.Addr.String()[index+1:]
	}
	// fmt.Println("metadata", m)
	return m, true
}

// QueryMetadata checks for valid credentials in the context Metadata.
func QueryMetadata(ctx context.Context, name string) (string, bool) {
	headers, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	if found, ok := headers[name]; ok {
		return found[0], true
	}
	if p, ok := peer.FromContext(ctx); ok {
		switch name {
		case "peer":
			return p.Addr.String(), true
		case "protocol":
			return p.Addr.Network(), true
		case "peer-address":
			index := strings.LastIndex(p.Addr.String(), ":")
			return p.Addr.String()[:index], true
		case "peer-port":
			index := strings.LastIndex(p.Addr.String(), ":")
			return p.Addr.String()[index+1:], true
		}
	}
	return "", true
}

// QueryAddress - for net.Addr interface
type QueryAddress struct {
	address string
	network string
}

// Network - for net.Addr interface
func (a QueryAddress) Network() string {
	return a.network
}

// String - for net.Addr interface
func (a QueryAddress) String() string {
	return a.address
}

// QueryAddr - Query Local and Remote Address from context
func QueryAddr(ctx context.Context) (net.Addr, net.Addr, bool) {
	peeraddr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, nil, false
	}
	headers, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil, false
	}
	found, ok := headers[":authority"]
	if !ok {
		return nil, nil, false
	}
	localaddr := QueryAddress{
		address: found[0],
		network: peeraddr.Addr.Network(),
	}
	return localaddr, peeraddr.Addr, true
}
