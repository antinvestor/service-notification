package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/util"
)

// ValidationResult holds the outcome of input validation.
type ValidationResult struct {
	Valid        bool
	ErrorMessage string
}

// InputValidator validates user input according to the menu's validator type.
type InputValidator struct {
	httpClient *http.Client
	regexCache sync.Map // menuID -> *regexp.Regexp
}

// NewInputValidator creates a validator with a shared HTTP client for external URL validation.
func NewInputValidator(httpClient *http.Client) *InputValidator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &InputValidator{httpClient: httpClient}
}

// Validate performs validation of userInput against the given menu item's rules.
func (iv *InputValidator) Validate(ctx context.Context, menu *models.UssdMenu, userInput string, childCount int, lang string) ValidationResult {
	logger := util.Log(ctx)

	// Regex check first (applies to all validators if set)
	if menu.Regex != "" {
		re, err := iv.getOrCompileRegex(menu.GetID(), menu.Regex)
		if err != nil {
			logger.WithError(err).Warn("invalid regex in menu item")
			return ValidationResult{Valid: false, ErrorMessage: getErrorMessage(menu, lang, "Invalid input")}
		}
		if !re.MatchString(userInput) {
			return ValidationResult{Valid: false, ErrorMessage: getErrorMessage(menu, lang, "Invalid input format")}
		}
	}

	switch menu.Validator {
	case models.ValidatorChoice:
		return iv.validateChoice(userInput, childCount, lang)
	case models.ValidatorInput:
		return iv.validateInput(userInput, lang)
	case models.ValidatorNumber:
		return iv.validateNumber(userInput, lang)
	case models.ValidatorDate:
		return iv.validateDate(userInput, lang)
	case models.ValidatorExternalURL:
		return iv.validateExternalURL(ctx, menu, userInput, lang)
	default:
		// No validator specified — accept any input
		return ValidationResult{Valid: true}
	}
}

func (iv *InputValidator) validateChoice(input string, childCount int, _ string) ValidationResult {
	if !isDigit(input) {
		return ValidationResult{Valid: false, ErrorMessage: "Please enter a valid number"}
	}
	n, _ := strconv.Atoi(input)
	if n < 1 || n > childCount {
		return ValidationResult{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("Please enter a number between 1 and %d", childCount),
		}
	}
	return ValidationResult{Valid: true}
}

func (iv *InputValidator) validateInput(input string, _ string) ValidationResult {
	if strings.TrimSpace(input) == "" {
		return ValidationResult{Valid: false, ErrorMessage: "Please enter a value"}
	}
	return ValidationResult{Valid: true}
}

func (iv *InputValidator) validateNumber(input string, _ string) ValidationResult {
	if !isDigit(input) {
		return ValidationResult{Valid: false, ErrorMessage: "Please enter a valid number"}
	}
	return ValidationResult{Valid: true}
}

func (iv *InputValidator) validateDate(input string, _ string) ValidationResult {
	if strings.TrimSpace(input) == "" {
		return ValidationResult{Valid: false, ErrorMessage: "Please enter a date in YYYY-MM-DD format"}
	}
	_, err := time.Parse("2006-01-02", input)
	if err != nil {
		return ValidationResult{Valid: false, ErrorMessage: "Invalid date. Use format YYYY-MM-DD"}
	}
	return ValidationResult{Valid: true}
}

func (iv *InputValidator) validateExternalURL(ctx context.Context, menu *models.UssdMenu, input string, lang string) ValidationResult {
	if r := iv.validateInput(input, lang); !r.Valid {
		return r
	}

	if menu.Extra == "" {
		return ValidationResult{Valid: true}
	}

	// SSRF guard: only allow HTTPS external validation URLs
	if !isAllowedValidationURL(menu.Extra) {
		util.Log(ctx).WithField("url", menu.Extra).Warn("rejected non-HTTPS external validation URL")
		return ValidationResult{Valid: false, ErrorMessage: "Validation service unavailable"}
	}

	logger := util.Log(ctx)

	payloadMap := map[string]string{
		"input_data":     input,
		"collection_key": menu.CollectionKey,
		"lang":           lang,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, menu.Extra, strings.NewReader(string(payloadBytes)))
	if err != nil {
		logger.WithError(err).Warn("failed to create external validation request")
		return ValidationResult{Valid: false, ErrorMessage: "Validation service unavailable"}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := iv.httpClient.Do(req)
	if err != nil {
		logger.WithError(err).Warn("external validation request failed")
		return ValidationResult{Valid: false, ErrorMessage: "Validation service unavailable"}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return ValidationResult{Valid: true}
	}

	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = "Invalid input"
		}
		return ValidationResult{Valid: false, ErrorMessage: msg}
	}

	return ValidationResult{Valid: false, ErrorMessage: "Validation service error"}
}

func (iv *InputValidator) getOrCompileRegex(menuID, pattern string) (*regexp.Regexp, error) {
	if cached, ok := iv.regexCache.Load(menuID); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	iv.regexCache.Store(menuID, re)
	return re, nil
}

// isAllowedValidationURL checks that external validation URLs use HTTPS
// and do not target private/loopback addresses (SSRF mitigation).
func isAllowedValidationURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	// Block obvious private/loopback targets
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return false
	}
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "169.254.") {
		return false
	}
	// Block 172.16.0.0/12
	if strings.HasPrefix(host, "172.") {
		parts := strings.SplitN(host, ".", 3)
		if len(parts) >= 2 {
			if octet, err := strconv.Atoi(parts[1]); err == nil && octet >= 16 && octet <= 31 {
				return false
			}
		}
	}
	return true
}

func getErrorMessage(menu *models.UssdMenu, _ string, fallback string) string {
	if menu.ErrorMessage != "" {
		return menu.ErrorMessage
	}
	return fallback
}

func isDigit(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
