package models

import (
	"context"
	"time"

	"github.com/pitabwire/frame/data"
)

// Action types define what happens at each menu node.
const (
	ActionChoices              = "CHOICES"
	ActionChoicesValue         = "CHOICES_VALUE"
	ActionChoicesValueContinue = "CHOICES_VALUE_CONTINUE"
	ActionChoicesCancel        = "CHOICES_CANCEL"
	ActionInput                = "INPUT"
	ActionInformation          = "INFORMATION"
	ActionInputFromQuery       = "INPUT_FROM_QUERY"
	ActionChoiceFromQuery      = "CHOICE_FROM_QUERY"
	ActionChoiceFromManyQuery  = "CHOICE_FROM_MANY_QUERY"
	ActionInverseInputFromQry  = "INVERSE_INPT_FRM_QRY"
	ActionInverseChoiceFromQry = "INVERSE_C_FRM_QRY"
	ActionInputFromURL         = "INPUT_FROM_URL"
	ActionInputFromCode        = "INPUT_FROM_CODE"
)

// Validator types define how user input is validated.
const (
	ValidatorChoice      = "CHOICE"
	ValidatorInput       = "INPUT"
	ValidatorNumber      = "NUMBER"
	ValidatorDate        = "DATE"
	ValidatorExternalURL = "EXTERNAL_URL"
)

// Session states track the lifecycle of a USSD session.
const (
	SessionStateActive   = "active"
	SessionStateInactive = "inactive"
)

// Navigation constants for special user inputs.
const (
	NavRestart  = "00"  // Restart from beginning
	NavRestartH = "#"   // Restart (hash shortcut)
	NavBack     = "0"   // Go back one level
	NavExit     = "000" // Exit session
)

// AllActions is the ordered list of valid action types.
var AllActions = []string{
	ActionChoices, ActionChoicesValue, ActionChoicesValueContinue, ActionChoicesCancel,
	ActionInput, ActionInformation,
	ActionInputFromQuery, ActionChoiceFromQuery, ActionChoiceFromManyQuery,
	ActionInverseInputFromQry, ActionInverseChoiceFromQry,
	ActionInputFromURL, ActionInputFromCode,
}

// AllValidators is the ordered list of valid validator types.
var AllValidators = []string{
	ValidatorChoice, ValidatorInput, ValidatorNumber, ValidatorDate, ValidatorExternalURL,
}

// IsChoiceAction returns true if the action requires choosing from a list.
func IsChoiceAction(action string) bool {
	switch action {
	case ActionChoices, ActionChoiceFromQuery, ActionChoiceFromManyQuery, ActionInverseChoiceFromQry:
		return true
	}
	return false
}

// IsQueryAction returns true if the action involves dynamic query data.
func IsQueryAction(action string) bool {
	switch action {
	case ActionInputFromQuery, ActionChoiceFromQuery, ActionChoiceFromManyQuery,
		ActionInverseInputFromQry, ActionInverseChoiceFromQry:
		return true
	}
	return false
}

// IsInverseAction returns true if the action shows content only when NO query match exists.
func IsInverseAction(action string) bool {
	switch action {
	case ActionInverseInputFromQry, ActionInverseChoiceFromQry:
		return true
	}
	return false
}

// IsTerminalAction returns true if the action ends the session flow.
func IsTerminalAction(action string) bool {
	switch action {
	case ActionChoicesValue, ActionChoicesCancel, ActionInformation:
		return true
	}
	return false
}

// UssdMenu represents a node in the USSD menu tree.
type UssdMenu struct {
	data.BaseModel

	OwnerID       string `gorm:"type:varchar(255);index"`
	ParentID      string `gorm:"type:varchar(50);index"`
	Order         int    `gorm:"default:0"`
	Action        string `gorm:"type:varchar(50);not null"`
	Validator     string `gorm:"type:varchar(50)"`
	Name          string `gorm:"type:varchar(160);not null"`
	Message       string `gorm:"type:varchar(320);not null"`
	CollectionKey string `gorm:"type:varchar(100)"`
	Extra         string `gorm:"type:varchar(500)"`
	Regex         string `gorm:"type:varchar(255)"`
	ErrorMessage  string `gorm:"type:varchar(255)"`
	IsPreference  bool   `gorm:"default:false"`
	IsPublic      bool   `gorm:"default:false"`
	IsActive      bool   `gorm:"default:true"`
}

func (UssdMenu) TableName() string {
	return "ussd_menus"
}

// UssdMenuFromParams creates a UssdMenu from provided parameters.
func UssdMenuFromParams(ctx context.Context, ownerID, parentID, action, validator, name, message, collectionKey, extra, regex, errorMessage string, isPreference, isPublic bool) *UssdMenu {
	m := &UssdMenu{
		OwnerID:       ownerID,
		ParentID:      parentID,
		Action:        action,
		Validator:     validator,
		Name:          name,
		Message:       message,
		CollectionKey: collectionKey,
		Extra:         extra,
		Regex:         regex,
		ErrorMessage:  errorMessage,
		IsPreference:  isPreference,
		IsPublic:      isPublic,
		IsActive:      true,
	}
	m.GenID(ctx)
	return m
}

// UssdTranslation stores localised text for a menu item.
type UssdTranslation struct {
	data.BaseModel

	MenuID       string `gorm:"type:varchar(50);uniqueIndex:uq_translation_menu_code;not null"`
	Code         string `gorm:"type:varchar(10);uniqueIndex:uq_translation_menu_code;not null"`
	Name         string `gorm:"type:varchar(160)"`
	Message      string `gorm:"type:varchar(320)"`
	ErrorMessage string `gorm:"type:varchar(255)"`
}

func (UssdTranslation) TableName() string {
	return "ussd_translations"
}

// UssdSession represents an active or completed USSD session.
type UssdSession struct {
	data.BaseModel

	MSISDN          string       `gorm:"type:varchar(20);not null;index:idx_session_lookup"`
	ServiceID       string       `gorm:"type:varchar(50);not null;index:idx_session_lookup"`
	SessionExternal string       `gorm:"type:varchar(100);index"`
	State           string       `gorm:"type:varchar(20);default:active;index:idx_session_lookup"`
	CurrentMenuID   string       `gorm:"type:varchar(50)"`
	PreviousMenuID  string       `gorm:"type:varchar(50)"`
	Language        string       `gorm:"type:varchar(10);default:en"`
	CollectedData   data.JSONMap `gorm:"type:jsonb"`
	ExpiresAt       time.Time    `gorm:"index:idx_session_lookup"`
}

func (UssdSession) TableName() string {
	return "ussd_sessions"
}

// IsExpired returns true if the session has passed its expiry time.
func (s *UssdSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// UssdQuery stores dynamic data used to populate query-based menus at runtime.
type UssdQuery struct {
	data.BaseModel

	MSISDN   string       `gorm:"type:varchar(20);index:idx_query_lookup"`
	UserID   string       `gorm:"type:varchar(50);index:idx_query_lookup"`
	BotID    string       `gorm:"type:varchar(50)"`
	Name     string       `gorm:"type:varchar(100);not null;index:idx_query_lookup"`
	Payload  data.JSONMap `gorm:"type:jsonb"`
	IsMany   bool         `gorm:"default:false"`
	IsActive bool         `gorm:"default:true;index:idx_query_lookup"`
}

func (UssdQuery) TableName() string {
	return "ussd_queries"
}

// UssdServiceConfig holds per-service USSD configuration (operator settings).
type UssdServiceConfig struct {
	data.BaseModel

	ServiceID   string `gorm:"type:varchar(50);uniqueIndex:uq_service_config;not null"`
	Name        string `gorm:"type:varchar(100);uniqueIndex:uq_service_config;not null"`
	Value       string `gorm:"type:varchar(500)"`
	Description string `gorm:"type:text"`
}

func (UssdServiceConfig) TableName() string {
	return "ussd_service_configs"
}

// Common service config keys.
const (
	ConfigDataField            = "data_field"
	ConfigMSISDNField          = "msisdn_field"
	ConfigSessionField         = "session_field"
	ConfigInitialMenu          = "initial_menu"
	ConfigOperatorCommandField = "operator_command_field"
	ConfigLangField            = "lang_field"
	ConfigDataBeginTemplate    = "data_begin_template"
	ConfigDataContinueTemplate = "data_continue_template"
	ConfigDataEndTemplate      = "data_end_template"
	ConfigServiceCodeField     = "service_code_field"
	ConfigServerCollectorKey   = "server_data_collector_key_field"
	ConfigAuthKey              = "auth_key"
	ConfigWhitelistIP          = "whitelist_ip"
	ConfigDefaultLang          = "default_lang"
	ConfigSessionExpiryMinutes = "session_expiry_minutes"
)
