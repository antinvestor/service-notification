package client

import (
	"fmt"
	"net/http"
)

// Meta Cloud API error codes the integration reacts to. See
// https://developers.facebook.com/docs/whatsapp/cloud-api/support/error-codes
const (
	codeAuthException          = 0
	codeAPIUnknown             = 1
	codeAPIService             = 2
	codePermissionDenied       = 10
	codeParameterInvalid       = 100
	codeAccessTokenExpired     = 190
	codeThroughputLimit        = 130429
	codeGenericUserError       = 131000
	codeParameterMissing       = 131008
	codeParameterValueInvalid  = 131009
	codeServiceUnavailable     = 131016
	codeRecipientIncapable     = 131026
	codeMessageUndeliverable   = 131047
	codeSpamRateLimit          = 131048
	codePairRateLimit          = 131056
	codeTemplateParamMissing   = 132000
	codeTemplateNotFound       = 132001
	codeTemplateParamsInvalid  = 132012
	codeReEngagementRequired   = 131047
	codeBusinessRateLimit      = 80007
	codeExperimentalRestricted = 131030
)

// FallbackChannel is the channel a WhatsApp delivery failure can be retried on.
const FallbackChannel = "sms"

// SendError is a failed Cloud API request classified for the queue worker.
type SendError struct {
	HTTPStatus int
	Code       int
	Subcode    int
	Type       string
	Message    string
	Details    string
	TraceID    string
}

func (e *SendError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("whatsapp api %d (code %d): %s: %s", e.HTTPStatus, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("whatsapp api %d (code %d): %s", e.HTTPStatus, e.Code, e.Message)
}

// Retriable reports whether re-sending the same message later may succeed.
func (e *SendError) Retriable() bool {
	switch e.Code {
	case codeThroughputLimit, codePairRateLimit, codeSpamRateLimit, codeBusinessRateLimit,
		codeAPIUnknown, codeAPIService, codeGenericUserError, codeServiceUnavailable:
		return true
	}
	if e.Code == 0 && e.HTTPStatus >= http.StatusInternalServerError {
		return true
	}
	return e.HTTPStatus == http.StatusTooManyRequests
}

// Fallback returns the channel to retry on when the recipient cannot be reached over
// WhatsApp at all (no WhatsApp account, or a free-form message outside the customer
// service window), or empty when falling back would not help.
func (e *SendError) Fallback() string {
	switch e.Code {
	case codeRecipientIncapable, codeMessageUndeliverable:
		return FallbackChannel
	}
	return ""
}

// Credential reports auth/permission failures that need operator attention rather than
// retries; the cached credentials are dropped so a rotated token is picked up.
func (e *SendError) Credential() bool {
	switch e.Code {
	case codeAuthException, codePermissionDenied, codeAccessTokenExpired:
		return e.HTTPStatus == http.StatusUnauthorized || e.HTTPStatus == http.StatusForbidden || e.Code != codeAuthException
	}
	return e.HTTPStatus == http.StatusUnauthorized
}

// StatusFallback maps a webhook "failed" status error code to a fallback channel.
func StatusFallback(code int) string {
	return (&SendError{Code: code}).Fallback()
}
