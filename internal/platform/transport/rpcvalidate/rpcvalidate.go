// Package rpcvalidate validates every Connect request against the
// protovalidate rules declared in contracts/connectrpc. The rules in the
// .proto files are the single definition of what a request must contain;
// handlers do not repeat those checks.
package rpcvalidate

import (
	"connectrpc.com/connect"
	"connectrpc.com/validate"
)

// HandlerOptions returns the Connect handler option that rejects a request
// breaking its message's rules with CodeInvalidArgument. The error carries
// the violations as a buf.validate.Violations detail.
func HandlerOptions() []connect.HandlerOption {
	return []connect.HandlerOption{connect.WithInterceptors(validate.NewInterceptor())}
}
