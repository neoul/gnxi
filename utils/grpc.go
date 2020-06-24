// Package utils implements utilities for gnxi.
package utils

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
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
	return m, true
}
