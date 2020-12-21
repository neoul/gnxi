package status

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// https://cloud.google.com/apis/design/errors

type grpcstatus interface {
	GRPCStatus() *status.Status
}

// type GStatus interface {
// 	Code() codes.Code
// 	Message() string
// 	ErrorReason() string
// 	ErrorPath() string
// 	Detail() []string
// }

// Status for rich gNMI error description
type Status struct {
	Code           codes.Code
	Message        string
	Tag            ErrorTag
	Reason         string // errdetails.ErrorInfo.Reason
	Domain         string // errdetails.ErrorInfo.Domain for ip addr
	Path           string // errdetails.ErrorInfo.Meta["Path"]
	SubError       []string
	badRequest     *errdetails.BadRequest          // codes.InvalidArgument and codes.OutOfRange
	precondFailure *errdetails.PreconditionFailure // FailedPrecondition
	resourceInfo   *errdetails.ResourceInfo        // NotFound, AlreadyExists
	quotaFailure   *errdetails.QuotaFailure        // ResourceExhausted
	errorInfo      *errdetails.ErrorInfo
}

func (s *Status) Error() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("[%s] %s", s.Tag, s.Message)
}

// // NewInvalidArgument return an error object including a BadRequest
// // NewInvalidArgument("invalid gnmi.set.update.value for 'path'", "gnmi.set.update.value", "value")
// func NewInvalidArgument(msg, field, desc string) error {
// 	s := New(codes.InvalidArgument, msg)
// 	v := &errdetails.BadRequest_FieldViolation{
// 		Field:       field,
// 		Description: desc,
// 	}
// 	br := &errdetails.BadRequest{}
// 	br.FieldViolations = append(br.FieldViolations, v)
// 	s.badRequest = br
// 	return s
// }

// // NewErrorInternal return an error object
// func NewErrorInternal(msg string) error {
// 	s := New(codes.Internal, msg)
// 	return s
// }

// // NewErrorfInternal return an error object
// func NewErrorfInternal(msg string, a ...interface{}) error {
// 	s := Newf(codes.Internal, msg, a...)
// 	return s
// }

// // ErrorInternal return an error object
// func ErrorInternal(err error) error {
// 	return Error(codes.Internal, err)
// }

// func (s *Status) Code() codes.Code {
// 	if s == nil {
// 		return codes.OK
// 	}
// 	return s.Code
// }

// func (s *Status) Message() string {
// 	if s == nil {
// 		return ""
// 	}
// 	return s.Message
// }

// func (s *Status) ErrorReason() string {
// 	if s == nil {
// 		return ""
// 	}
// 	return s.Reason
// }

// func (s *Status) ErrorPath() string {
// 	if s == nil {
// 		return ""
// 	}
// 	return s.Path
// }

// func (s *Status) SubErrors() []string {
// 	if s == nil {
// 		return nil
// 	}
// 	return s.SubError
// }

func (s *Status) String() string {
	if s == nil {
		return ""
	}
	// b, err := json.Marshal(s)
	// if err != nil {
	// 	return ""
	// }
	// rstr := string(b)
	rstr := fmt.Sprintf(`
	status:
	 Code: %s
	 Message: %s
	 Reason: %s
	 Domain: %s
	 Path: %s
	 SubError:
	`, s.Code, s.Message, s.Reason, s.Domain, s.Path,
	)
	for i := range s.SubError {
		rstr = fmt.Sprintf("%s  - %s\n", rstr, s.SubError[i])
	}
	return rstr
}

// GRPCStatus returns gRPC status
func (s *Status) GRPCStatus() *status.Status {
	msg := fmt.Sprintf("[%s] %s", s.Tag, s.Message)
	ss := status.New(s.Code, msg)
	if s.badRequest != nil {
		ss.WithDetails(s.badRequest)
	}
	if s.Reason != "" || s.Domain != "" || s.Path != "" || s.SubError != nil {
		m := map[string]string{
			"Path": s.Path,
		}
		for i := range s.SubError {
			m[fmt.Sprintf("error%d", i)] = s.SubError[i]
		}
		ss.WithDetails(&errdetails.ErrorInfo{
			Reason:   s.Reason,
			Domain:   s.Domain,
			Metadata: m,
		})
	}
	return ss
}

// ToProto return gRPC error status
func ToProto(err error) *spb.Status {
	if err == nil {
		return nil
	}
	s := FromError(err)
	return s.GRPCStatus().Proto()
}

func EmptyProto() *spb.Status {
	return &spb.Status{}
}

// New returns a Status representing c and msg.
func New(c codes.Code, tag ErrorTag, msg string) *Status {
	s := &Status{
		Code:    c,
		Message: msg,
		Tag:     tag,
	}
	return s
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c codes.Code, tag ErrorTag, format string, a ...interface{}) *Status {
	return New(c, tag, fmt.Sprintf(format, a...))
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c codes.Code, err error) error {
	s := FromError(err)
	if s == nil {
		return s
	}
	if s.Code == codes.Unknown || s.Code == codes.OK {
		s.Code = c
	}
	return s
}

// // Errorb returns new GStatus based on input errors.
// func Errorb(err error, c codes.Code, format string, a ...interface{}) error {
// 	inerr := FromError(err)
// 	if inerr == nil {
// 		return New(c, "", fmt.Sprintf(format, a...))
// 	}
// 	if inerr.Code == codes.Unknown || inerr.Code == codes.OK {
// 		inerr.Code = c
// 	}
// 	inerr.SubError = append(inerr.SubError, inerr.Message)
// 	inerr.Message = fmt.Sprintf(format, a...)
// 	return inerr
// }

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return New(c, 0, fmt.Sprintf(format, a...))
}

// ErrorTag for simple error indication
type ErrorTag int

const (
	// TagUnknown - ErrorTag
	TagUnknown ErrorTag = iota
	// TagMissingPath - invalid path
	TagMissingPath
	// TagInvalidPath - ErrorTag
	TagInvalidPath
	// TagUnknownPath - ErrorTag
	TagUnknownPath
	// TagInvalidAlias - ErrorTag
	TagInvalidAlias
	// TagDataMissing - ErrorTag
	TagDataMissing
	// TagBadData - ErrorTag
	TagBadData
	// TagNotSupport - ErrorTag
	TagNotSupport
	// TagOperationFail - ErrorTag
	TagOperationFail
	// TagMalformedMessage - ErrorTag
	TagMalformedMessage
	// TagInvalidConfig - ErrorTag
	TagInvalidConfig
	// TagInvalidSession - ErrorTag
	TagInvalidSession

	tagMax
)

var tagStr map[ErrorTag]string = map[ErrorTag]string{
	TagMissingPath:      "missing-path",
	TagInvalidPath:      "invalid-path",
	TagUnknownPath:      "unknown-path",
	TagInvalidAlias:     "invalid-path",
	TagDataMissing:      "data-missing",
	TagBadData:          "bad-data",
	TagNotSupport:       "not-support",
	TagOperationFail:    "operation-fail",
	TagMalformedMessage: "malformed-message",
	TagInvalidConfig:    "invalid-config",
	TagInvalidSession:   "invalid-session",
}

func (t ErrorTag) String() string {
	if s, ok := tagStr[t]; ok {
		return s
	}
	return "unknown-error"
}

// TaggedError returns a tagged error
// Tag:
//  - invalid-path
//  - invalid-alias
//  - data-missing
//  - in-use
//  - invalid-value
//  - too-big
//  - missing-attribute
//  - bad-attribute
//  - unknown-attribute
//  - missing-element
//  - bad-element
//  - unknown-element
//  - access-denied
//  - unknown-namespace
//  - lock-denied
//  - resource-denied
//  - rollback-failed
//  - data-exists
//  - operation-not-supported
//  - operation-failed
//  - partial-operation
//  - malformed-message
func TaggedError(c codes.Code, tag ErrorTag, err error) error {
	s := FromError(err)
	if s == nil {
		return s
	}
	if s.Code == codes.Unknown || s.Code == codes.OK {
		s.Code = c
	}
	if s.Tag >= tagMax {
		s.Tag = tag
	}
	return s
}

// TaggedErrorf returns a tagged error
func TaggedErrorf(c codes.Code, tag ErrorTag, format string, a ...interface{}) error {
	return New(c, tag, fmt.Sprintf(format, a...))
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
// Status is returned with codes.Unknown and the original error Message.
func FromError(err error) *Status {
	if err == nil {
		// s := New(codes.OK, "")
		// return s
		return nil
	}
	if s, ok := err.(*Status); ok {
		return s
	}
	if gs, ok := err.(grpcstatus); ok {
		s := New(gs.GRPCStatus().Code(), TagUnknown, gs.GRPCStatus().Message())
		return s
	}
	return New(codes.Unknown, TagUnknown, err.Error())
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
	if s, ok := err.(*Status); ok {
		return s.Code
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
	if s, ok := err.(*Status); ok {
		return s.Message
	}
	if gs, ok := err.(grpcstatus); ok {
		return gs.GRPCStatus().Message()
	}
	return err.Error()
}

// FromContextError converts a context error into a Status.  It returns a
// Status with codes.OK if err is nil, or a Status with codes.Unknown if err is
// non-nil and not a context error.
func FromContextError(err error) error {
	switch err {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return New(codes.DeadlineExceeded, TagUnknown, err.Error())
	case context.Canceled:
		return New(codes.Canceled, TagUnknown, err.Error())
	default:
		return New(codes.Unknown, TagUnknown, err.Error())
	}
}

// // WithErrorInfo adds the error detail to the Status
// func (s *Status) WithErrorInfo(Reason, Path string) error {
// 	s.Reason = Reason
// 	s.Path
// 	st, err := s.WithDetails(&epb.ErrorInfo{
// 		Reason:   Reason,
// 		Domain:   Domain,
// 		Metadata: meta,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	s.Status = st
// 	return nil
// }

// // WithErrorPath adds the error Path information to the Status
// func (s *Status) WithErrorPath(Reason, Domain, errpath string) error {
// 	st, err := s.WithDetails(&epb.ErrorInfo{
// 		Reason:   Reason,
// 		Domain:   Domain,
// 		Metadata: map[string]string{"Path": errpath},
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
// 		return fmt.Sprintf("rpc error:\n Code = %s\n", Code(err))
// 	}
// 	gs, ok := err.(grpcstatus)
// 	if !ok {
// 		return fmt.Sprintf("rpc error:\n Code = %s\n", Code(err))
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
// 			output += fmt.Sprintf("  Reason: %s\n", t.GetReason())
// 			output += fmt.Sprintf("  Domain: %s\n", t.GetDomain())
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
// 	return fmt.Sprintf("rpc error:\n Code = %s\n desc = %s\n%s",
// 		codes.Code(s.Code()), s.Message(), output)
// }

// // ErrorPath returns the error Path of the Status
// func (s *Status) ErrorPath() string {
// 	for _, detail := range s.Details() {
// 		switch t := detail.(type) {
// 		case *errdetails.ErrorInfo:
// 			for k, v := range t.GetMetadata() {
// 				if k == "Path" {
// 					return v
// 				}
// 			}
// 		}
// 	}
// 	return ""
// }

// UpdateResultError returns s's status as an spb.Status proto Message.
func UpdateResultError(err error) *gnmipb.Error {
	if err == nil {
		return nil
	}
	s := FromError(err)
	e := &gnmipb.Error{
		Code:    uint32(s.Code),
		Message: fmt.Sprintf("[%s] %s", s.Tag, s.Message),
	}
	if s.Reason != "" || s.Domain != "" || s.Path != "" || s.SubError != nil {
		m := map[string]string{
			"Path": s.Path,
		}
		for i := range s.SubError {
			m[fmt.Sprintf("error%d", i)] = s.SubError[i]
		}
		einfo := &errdetails.ErrorInfo{
			Reason:   s.Reason,
			Domain:   s.Domain,
			Metadata: m,
		}
		if any, err := ptypes.MarshalAny(einfo); err == nil {
			e.Data = any
		}
	}
	return e
}

func WithErrorPath(err error, Path string) error {
	s := FromError(err)
	s.Path = Path
	return err
}

func WithErrorReason(err error, Reason string) error {
	s := FromError(err)
	s.Reason = Reason
	return err
}

func WithDomain(err error, Domain string) error {
	s := FromError(err)
	s.Domain = Domain
	return err
}

func WithInfo(err error, Reason, Path, Domain string) error {
	s := FromError(err)
	s.Reason = Reason
	s.Path = Path
	s.Domain = Domain
	return err
}
