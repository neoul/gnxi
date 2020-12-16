package status

import (
	"context"
	"fmt"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcstatus interface {
	GRPCStatus() *status.Status
}

type GStatus interface {
	Code() codes.Code
	Message() string
	ErrorReason() string
	ErrorPath() string
	Detail() []string
}

// Status for rich gNMI error description
type Status struct {
	code    codes.Code
	message string
	reason  string // errdetails.ErrorInfo.Reason
	domain  string // errdetails.ErrorInfo.Domain for ip addr
	path    string // errdetails.ErrorInfo.Meta["path"]
	detail  []string
}

func (s *Status) Error() string {
	if s == nil {
		return ""
	}
	return s.message
}

func (s *Status) Code() codes.Code {
	if s == nil {
		return codes.OK
	}
	return s.code
}

func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return s.message
}

func (s *Status) ErrorReason() string {
	if s == nil {
		return ""
	}
	return s.reason
}

func (s *Status) ErrorPath() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Status) Detail() []string {
	if s == nil {
		return nil
	}
	return s.detail
}

// GRPCStatus returns gRPC status
func (s *Status) GRPCStatus() *status.Status {
	ss := status.New(s.code, s.message)
	if s.reason != "" || s.domain != "" || s.path != "" || s.detail != nil {
		m := map[string]string{
			"path": s.path,
		}
		for i := range s.detail {
			m[fmt.Sprintf("error%d", i)] = s.detail[i]
		}
		ss.WithDetails(&errdetails.ErrorInfo{
			Reason:   s.reason,
			Domain:   s.domain,
			Metadata: m,
		})
	}
	return ss
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	s := &Status{
		code:    c,
		message: msg,
	}
	return s
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c codes.Code, format string, a ...interface{}) GStatus {
	return New(c, fmt.Sprintf(format, a...))
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c codes.Code, err error) error {
	if err == nil {
		return nil
	}
	if s, ok := err.(grpcstatus); ok {
		s := &Status{
			code:    s.GRPCStatus().Code(),
			message: s.GRPCStatus().Message(),
		}
		return s
	}
	return New(c, err.Error())
}

// Errorb returns new GStatus based on input errors.
func Errorb(err error, c codes.Code, format string, a ...interface{}) error {
	inputerr := FromError(err)
	e := New(c, fmt.Sprintf(format, a...))
	if inputerr != nil {
		if inputerr.Code() != codes.Unknown && inputerr.Code() != codes.OK {
			e.code = inputerr.Code()
			e.reason = inputerr.ErrorReason()
			e.path = inputerr.ErrorPath()
			e.detail = inputerr.Detail()
			e.detail = append(e.detail,
				fmt.Sprintf("rpc error: code = %s desc = %s",
					codes.Code(inputerr.Code()), inputerr.Message()))
		}
	}
	return e
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return New(c, fmt.Sprintf(format, a...))
}

// // ErrorProto returns an error representing s.  If s.Code is OK, returns nil.
// func ErrorProto(s *spb.Status) error {
// 	return FromProto(s).Err()
// }

// // FromProto returns a Status representing s.
// func FromProto(s *spb.Status) *Status {
// 	st := status.FromProto(s)
// 	return &Status{Status: st}
// }

// FromError returns a Status representing err if it was produced from this
// package or has a method `GRPCStatus() *Status`. Otherwise, ok is false and a
// Status is returned with codes.Unknown and the original error message.
func FromError(err error) GStatus {
	if err == nil {
		// s := New(codes.OK, "")
		// return s
		return nil
	}
	if s, ok := err.(GStatus); ok {
		return s
	}
	if gs, ok := err.(grpcstatus); ok {
		s := New(gs.GRPCStatus().Code(), gs.GRPCStatus().Message())
		return s
	}
	return New(codes.Unknown, err.Error())
}

// // Convert is a convenience function which removes the need to handle the
// // boolean return value from FromError.
// func Convert(err error) *Status {
// 	s, _ := FromError(err)
// 	return s
// }

// Code returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise.
func Code(err error) codes.Code {
	// Don't use FromError to avoid allocation of OK status.
	if err == nil {
		return codes.OK
	}
	if s, ok := err.(GStatus); ok {
		return s.Code()
	}
	if gs, ok := err.(grpcstatus); ok {
		return gs.GRPCStatus().Code()
	}
	return codes.Unknown
}

// Message returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise.
func Message(err error) string {
	// Don't use FromError to avoid allocation of OK status.
	if err == nil {
		return ""
	}
	if s, ok := err.(GStatus); ok {
		return s.Message()
	}
	if gs, ok := err.(grpcstatus); ok {
		return gs.GRPCStatus().Message()
	}
	return err.Error()
}

// FromContextError converts a context error into a Status.  It returns a
// Status with codes.OK if err is nil, or a Status with codes.Unknown if err is
// non-nil and not a context error.
func FromContextError(err error) GStatus {
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

// // WithErrorInfo adds the error detail to the Status
// func (s *Status) WithErrorInfo(reason, path string) error {
// 	s.Reason = reason
// 	s.Path
// 	st, err := s.WithDetails(&epb.ErrorInfo{
// 		Reason:   reason,
// 		Domain:   domain,
// 		Metadata: meta,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	s.Status = st
// 	return nil
// }

// // WithErrorPath adds the error path information to the Status
// func (s *Status) WithErrorPath(reason, domain, errpath string) error {
// 	st, err := s.WithDetails(&epb.ErrorInfo{
// 		Reason:   reason,
// 		Domain:   domain,
// 		Metadata: map[string]string{"path": errpath},
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	s.Status = st
// 	return nil
// }

// // ErrorDetail add error detail to the Status
// func ErrorDetail(err error) string {
// 	var output string
// 	if err == nil {
// 		return fmt.Sprintf("rpc error:\n code = %s\n", Code(err))
// 	}
// 	gs, ok := err.(grpcstatus)
// 	if !ok {
// 		return fmt.Sprintf("rpc error:\n code = %s\n", Code(err))
// 	}
// 	if gs, ok := err.(grpcstatus); ok {

// 	}
// 	for _, detail := range s.Details() {
// 		switch t := detail.(type) {
// 		case *errdetails.BadRequest:
// 			output += fmt.Sprintf(" bad-request:\n")
// 			for _, violation := range t.GetFieldViolations() {
// 				output += fmt.Sprintf("  - %q: %s\n", violation.GetField(), violation.GetDescription())
// 			}
// 		case *errdetails.ErrorInfo:
// 			output += fmt.Sprintf(" error-info:\n")
// 			output += fmt.Sprintf("  reason: %s\n", t.GetReason())
// 			output += fmt.Sprintf("  domain: %s\n", t.GetDomain())
// 			if len(t.Metadata) > 0 {
// 				output += fmt.Sprintf("  meta-data:\n")
// 				for k, v := range t.GetMetadata() {
// 					output += fmt.Sprintf("   %s: %s\n", k, v)
// 				}
// 			}
// 		case *errdetails.RetryInfo:
// 			output += fmt.Sprintf(" retry-delay: %s\n", t.GetRetryDelay())
// 		case *errdetails.ResourceInfo:
// 			output += fmt.Sprintf(" resouce-info:\n")
// 			output += fmt.Sprintf("  name: %s\n", t.GetResourceName())
// 			output += fmt.Sprintf("  type: %s\n", t.GetResourceType())
// 			output += fmt.Sprintf("  owner: %s\n", t.GetOwner())
// 			output += fmt.Sprintf("  desc: %s\n", t.GetDescription())
// 		case *errdetails.RequestInfo:
// 			output += fmt.Sprintf(" request-info:\n")
// 			output += fmt.Sprintf("  %s: %s\n", t.GetRequestId(), t.GetServingData())
// 		}
// 	}
// 	return fmt.Sprintf("rpc error:\n code = %s\n desc = %s\n%s",
// 		codes.Code(s.Code()), s.Message(), output)
// }

// // ErrorPath returns the error path of the Status
// func (s *Status) ErrorPath() string {
// 	for _, detail := range s.Details() {
// 		switch t := detail.(type) {
// 		case *errdetails.ErrorInfo:
// 			for k, v := range t.GetMetadata() {
// 				if k == "path" {
// 					return v
// 				}
// 			}
// 		}
// 	}
// 	return ""
// }

// UpdateResultError returns s's status as an spb.Status proto message.
func UpdateResultError(err error) *gnmipb.Error {
	if err == nil {
		return nil
	}
	s := FromError(err)
	e := &gnmipb.Error{
		Code:    uint32(s.Code()),
		Message: s.Message(),
	}
	// if len(s.Status.Proto().Details) > 0 {
	// 	e.Data = s.Status.Proto().Details[0]
	// }
	return e
}
