package engine

import "strings"

// FlowOutcome indicates how a USSD turn ended.
type FlowOutcome int

const (
	// OutcomeContinue means the session continues and a response should be sent.
	OutcomeContinue FlowOutcome = iota
	// OutcomeDone means data collection is complete and the session terminates.
	OutcomeDone
	// OutcomeDoneContinue means data collection is complete but the session resets to start.
	OutcomeDoneContinue
	// OutcomeTerminate means the session must end (goodbye).
	OutcomeTerminate
	// OutcomeCanceled means the user or system canceled the session.
	OutcomeCanceled
)

// ProcessingResult is the typed return value from processing a USSD turn.
// It replaces the Python exception-based flow control with explicit values.
type ProcessingResult struct {
	Outcome       FlowOutcome
	Message       string
	CollectionKey string
	CollectedData string
	IsPreference  bool
	NextMenuID    string
	PrevMenuID    string
	SessionID     string
}

// Response is the final message sent back to the telco gateway.
type Response struct {
	Message   string
	IsEnd     bool
	SessionID string
}

// ResponseBuilder constructs the final user-facing response.
type ResponseBuilder struct {
	beginTemplate    string
	continueTemplate string
	endTemplate      string
}

// NewResponseBuilder creates a builder with the configured templates.
func NewResponseBuilder(beginTpl, continueTpl, endTpl string) *ResponseBuilder {
	return &ResponseBuilder{
		beginTemplate:    beginTpl,
		continueTemplate: continueTpl,
		endTemplate:      endTpl,
	}
}

// Build applies the correct template and returns the final response.
func (rb *ResponseBuilder) Build(message string, isBeginning, isEnd bool) Response {
	tpl := rb.continueTemplate
	if isBeginning {
		tpl = rb.beginTemplate
	}
	if isEnd {
		tpl = rb.endTemplate
	}

	finalMessage := message
	if tpl != "" && strings.Contains(tpl, "%s") {
		finalMessage = strings.Replace(tpl, "%s", message, 1)
	}

	return Response{
		Message: finalMessage,
		IsEnd:   isEnd,
	}
}

// MenuDisplay holds the rendered text and metadata for one USSD screen.
type MenuDisplay struct {
	Message       string
	Validator     string
	ErrorMessage  string
	NextMenuID    string
	PrevMenuID    string
	CollectionKey string
	IsPreference  bool
}
