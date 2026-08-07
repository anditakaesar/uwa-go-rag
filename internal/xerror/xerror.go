package xerror

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorSession represents authentication or session-related issues
type ErrorSession struct {
	Message string
}

func (e *ErrorSession) Error() string { return e.Message }

// ErrorNotFound represents missing resources
type ErrorNotFound struct {
	Message string
}

func (e *ErrorNotFound) Error() string { return e.Message }

// ErrorPermission represents authorization (RBAC) issues
type ErrorPermission struct {
	Message string
}

func (e *ErrorPermission) Error() string { return e.Message }

// ErrorBadRequest represents client request errors
type ErrorBadRequest struct {
	Message string
}

func (e *ErrorBadRequest) Error() string { return e.Message }

// ErrorToken represents token-related issues
type ErrorToken struct {
	Message string
}

func (e *ErrorToken) Error() string { return e.Message }

// ErrorValidation represents client request validation issues
type ErrorValidation struct {
	Message string
}

func (e *ErrorValidation) Error() string { return e.Message }

// ErrorDecodingRequest represents client request decoding
type ErrorDecodingRequest struct {
	Err error
}

func (e *ErrorDecodingRequest) Error() string {
	return fmt.Sprintf("error while decoding request: %v", e.Err)
}

type ErrorAuditLogRecordValidation struct {
	Message string
}

func (e *ErrorAuditLogRecordValidation) Error() string {
	return e.Message
}

type ErrorResourceNotFound struct {
	Message string
}

func (e *ErrorResourceNotFound) Error() string {
	return e.Message
}

type ErrorPasswordAttempt struct {
	Message string
}

func (e *ErrorPasswordAttempt) Error() string {
	return e.Message
}

type ErrorPathParamValue struct {
	Message      string
	ExpectedName string
}

func (e *ErrorPathParamValue) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return fmt.Sprintf("%s, expected param name: %s", e.Message, e.ExpectedName)
	}

	return fmt.Sprintf("expected param name: %s", e.ExpectedName)
}

// DefineStatusCode maps custom error types to HTTP Status Codes
func DefineStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Use errors.As to detect wrapped errors or specific types
	var errSession *ErrorSession
	var errPassAtp *ErrorPasswordAttempt
	if errors.As(err, &errSession) ||
		errors.As(err, &errPassAtp) {
		return http.StatusUnauthorized
	}

	var errPermission *ErrorPermission
	if errors.As(err, &errPermission) {
		return http.StatusForbidden
	}

	var errNotFound *ErrorNotFound
	var errResourceNotFound *ErrorResourceNotFound
	var errPathParamValue *ErrorPathParamValue
	if errors.As(err, &errNotFound) ||
		errors.As(err, &errResourceNotFound) ||
		errors.As(err, &errPathParamValue) {
		return http.StatusNotFound
	}

	var errBadRequest *ErrorBadRequest
	var errValidation *ErrorValidation
	var errDecodingReq *ErrorDecodingRequest
	if errors.As(err, &errBadRequest) ||
		errors.As(err, &errValidation) ||
		errors.As(err, &errDecodingReq) {
		return http.StatusBadRequest
	}

	// Fallback for everything else
	return http.StatusInternalServerError
}
