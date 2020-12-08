package status

import (
	"context"
	"fmt"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Status for gNMI error
type Status struct {
	*status.Status
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	s := status.New(c, msg)
	return &Status{Status: s}
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c codes.Code, format string, a ...interface{}) *Status {
	return New(c, fmt.Sprintf(format, a...))
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c codes.Code, err error) error {
	if err == nil {
		return nil
	}
	if se, ok := err.(interface {
		GRPCStatus() *Status
	}); ok {
		st := se.GRPCStatus()
		st.Proto().Code = int32(c)
		return st.Err()
	}
	return New(c, err.Error()).Err()
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return New(c, fmt.Sprintf(format, a...)).Err()
}

// ErrorProto returns an error representing s.  If s.Code is OK, returns nil.
func ErrorProto(s *spb.Status) error {
	return FromProto(s).Err()
}

// FromProto returns a Status representing s.
func FromProto(s *spb.Status) *Status {
	st := status.FromProto(s)
	return &Status{Status: st}
}

// FromError returns a Status representing err if it was produced from this
// package or has a method `GRPCStatus() *Status`. Otherwise, ok is false and a
// Status is returned with codes.Unknown and the original error message.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}
	if se, ok := err.(interface {
		GRPCStatus() *Status
	}); ok {
		return se.GRPCStatus(), true
	}
	return New(codes.Unknown, err.Error()), false
}

// Convert is a convenience function which removes the need to handle the
// boolean return value from FromError.
func Convert(err error) *Status {
	s, _ := FromError(err)
	return s
}

// Code returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise.
func Code(err error) codes.Code {
	// Don't use FromError to avoid allocation of OK status.
	if err == nil {
		return codes.OK
	}
	if se, ok := err.(interface {
		GRPCStatus() *Status
	}); ok {
		return se.GRPCStatus().Code()
	}
	return codes.Unknown
}

// FromContextError converts a context error into a Status.  It returns a
// Status with codes.OK if err is nil, or a Status with codes.Unknown if err is
// non-nil and not a context error.
func FromContextError(err error) *Status {
	switch err {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return New(codes.DeadlineExceeded, err.Error())
	case context.Canceled:
		return New(codes.Canceled, err.Error())
	default:
		return New(codes.Unknown, err.Error())
	}
}

// WithErrorInfo adds the error detail to the Status
func (s *Status) WithErrorInfo(reason, domain string, meta map[string]string) error {
	st, err := s.WithDetails(&epb.ErrorInfo{
		Reason:   reason,
		Domain:   domain,
		Metadata: meta,
	})
	if err != nil {
		return err
	}
	s.Status = st
	return nil
}

// WithErrorPath adds the error path information to the Status
func (s *Status) WithErrorPath(reason, domain, errpath string) error {
	st, err := s.WithDetails(&epb.ErrorInfo{
		Reason:   reason,
		Domain:   domain,
		Metadata: map[string]string{"path": errpath},
	})
	if err != nil {
		return err
	}
	s.Status = st
	return nil
}

// ErrorDetail add error detail to the Status
func (s *Status) ErrorDetail() string {
	var output string
	for _, detail := range s.Details() {
		switch t := detail.(type) {
		case *errdetails.BadRequest:
			output += fmt.Sprintf(" bad-request:\n")
			for _, violation := range t.GetFieldViolations() {
				output += fmt.Sprintf("  - %q: %s\n", violation.GetField(), violation.GetDescription())
			}
		case *errdetails.ErrorInfo:
			output += fmt.Sprintf(" error-info:\n")
			output += fmt.Sprintf("  reason: %s\n", t.GetReason())
			output += fmt.Sprintf("  domain: %s\n", t.GetDomain())
			if len(t.Metadata) > 0 {
				output += fmt.Sprintf("  meta-data:\n")
				for k, v := range t.GetMetadata() {
					output += fmt.Sprintf("   %s: %s\n", k, v)
				}
			}
		case *errdetails.RetryInfo:
			output += fmt.Sprintf(" retry-delay: %s\n", t.GetRetryDelay())
		case *errdetails.ResourceInfo:
			output += fmt.Sprintf(" resouce-info:\n")
			output += fmt.Sprintf("  name: %s\n", t.GetResourceName())
			output += fmt.Sprintf("  type: %s\n", t.GetResourceType())
			output += fmt.Sprintf("  owner: %s\n", t.GetOwner())
			output += fmt.Sprintf("  desc: %s\n", t.GetDescription())
		case *errdetails.RequestInfo:
			output += fmt.Sprintf(" request-info:\n")
			output += fmt.Sprintf("  %s: %s\n", t.GetRequestId(), t.GetServingData())
		}
	}
	return fmt.Sprintf("rpc error:\n code = %s\n desc = %s\n%s",
		codes.Code(s.Code()), s.Message(), output)
}

// ErrorPath returns the error path of the Status
func (s *Status) ErrorPath() string {
	for _, detail := range s.Details() {
		switch t := detail.(type) {
		case *errdetails.ErrorInfo:
			for k, v := range t.GetMetadata() {
				if k == "path" {
					return v
				}
			}
		}
	}
	return ""
}

// UpdateError returns s's status as an spb.Status proto message.
func (s *Status) UpdateError() *gnmipb.Error {
	if s == nil {
		return nil
	}
	e := &gnmipb.Error{
		Code:    uint32(s.Code()),
		Message: s.Message(),
	}
	if len(s.Status.Proto().Details) > 0 {
		e.Data = s.Status.Proto().Details[0]
	}
	return e
}
