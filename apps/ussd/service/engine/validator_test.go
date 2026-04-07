package engine

import (
	"context"
	"testing"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/stretchr/testify/require"
)

func TestValidateChoice(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{
		Validator: models.ValidatorChoice,
	}

	tests := []struct {
		name       string
		input      string
		childCount int
		wantValid  bool
	}{
		{"valid choice 1", "1", 3, true},
		{"valid choice 3", "3", 3, true},
		{"zero", "0", 3, false},
		{"too high", "4", 3, false},
		{"not a number", "abc", 3, false},
		{"empty", "", 3, false},
		{"negative", "-1", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := iv.Validate(ctx, menu, tt.input, tt.childCount, "en")
			require.Equal(t, tt.wantValid, r.Valid)
		})
	}
}

func TestValidateNumber(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{
		Validator: models.ValidatorNumber,
	}

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid number", "12345", true},
		{"zero", "0", true},
		{"text", "abc", false},
		{"mixed", "12a", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := iv.Validate(ctx, menu, tt.input, 0, "en")
			require.Equal(t, tt.wantValid, r.Valid)
		})
	}
}

func TestValidateDate(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{
		Validator: models.ValidatorDate,
	}

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid date", "2024-01-15", true},
		{"invalid format", "15/01/2024", false},
		{"not a date", "abc", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := iv.Validate(ctx, menu, tt.input, 0, "en")
			require.Equal(t, tt.wantValid, r.Valid)
		})
	}
}

func TestValidateInput(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{
		Validator: models.ValidatorInput,
	}

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid text", "hello", true},
		{"number as text", "123", true},
		{"empty", "", false},
		{"spaces only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := iv.Validate(ctx, menu, tt.input, 0, "en")
			require.Equal(t, tt.wantValid, r.Valid)
		})
	}
}

func TestRegexValidation(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{
		Validator: models.ValidatorInput,
		Regex:     `^\d{4}$`,
	}

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"4 digits", "1234", true},
		{"3 digits", "123", false},
		{"5 digits", "12345", false},
		{"letters", "abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := iv.Validate(ctx, menu, tt.input, 0, "en")
			require.Equal(t, tt.wantValid, r.Valid)
		})
	}
}

func TestNoValidator(t *testing.T) {
	iv := NewInputValidator(nil)
	ctx := context.Background()

	menu := &models.UssdMenu{} // No validator set

	r := iv.Validate(ctx, menu, "anything", 0, "en")
	require.True(t, r.Valid)
}
