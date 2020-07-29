package server

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errMissingUsername = status.Errorf(codes.InvalidArgument, "missing username")
	errMissingpassword = status.Errorf(codes.InvalidArgument, "missing password")
	errAuthFailed      = status.Errorf(codes.Unauthenticated, "authentication failed")
	errInvalidSession  = status.Errorf(codes.InvalidArgument, "invalid session")
)
