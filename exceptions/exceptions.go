package exceptions

import "fmt"

// AntherError is the base exception for all anther library errors.
type AntherError struct {
	Message string
}

func (e *AntherError) Error() string {
	return e.Message
}

// TransactionError represents an error during transaction operations.
type TransactionError struct {
	*AntherError
}

// NewTransactionError creates a new TransactionError.
func NewTransactionError(msg string) *TransactionError {
	return &TransactionError{
		AntherError: &AntherError{Message: msg},
	}
}

// MissingKeyError represents an error when a key is missing.
type MissingKeyError struct {
	*AntherError
}

// NewMissingKeyError creates a new MissingKeyError.
func NewMissingKeyError(account, role string) *MissingKeyError {
	return &MissingKeyError{
		AntherError: &AntherError{
			Message: fmt.Sprintf("No %s key for account '%s'", role, account),
		},
	}
}

// InvalidKeyFormatError represents an error when a key has an invalid format.
type InvalidKeyFormatError struct {
	*AntherError
}

// NewInvalidKeyFormatError creates a new InvalidKeyFormatError.
func NewInvalidKeyFormatError(msg string) *InvalidKeyFormatError {
	return &InvalidKeyFormatError{
		AntherError: &AntherError{Message: msg},
	}
}

// NodeError represents an error from a Hive node.
type NodeError struct {
	*AntherError
}

// NewNodeError creates a new NodeError.
func NewNodeError(msg string) *NodeError {
	return &NodeError{
		AntherError: &AntherError{Message: msg},
	}
}

// RPCError represents a JSON-RPC error returned by a Hive node.
type RPCError struct {
	*AntherError
	Code int
}

// NewRPCError creates a new RPCError.
func NewRPCError(code int, msg string) *RPCError {
	return &RPCError{
		AntherError: &AntherError{Message: msg},
		Code:        code,
	}
}

// SerializationError represents a local serialization or deserialization error.
type SerializationError struct {
	*AntherError
}

// NewSerializationError creates a new SerializationError.
func NewSerializationError(msg string) *SerializationError {
	return &SerializationError{
		AntherError: &AntherError{Message: msg},
	}
}
