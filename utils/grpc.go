// Package utils implements utilities for gnxi.
package utils

import (
	"context"
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
