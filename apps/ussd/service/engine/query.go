package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/util"
)

// QueryResolver expands menu items that depend on dynamic query data.
// This handles CHOICE_FROM_QUERY, INPUT_FROM_QUERY, CHOICE_FROM_MANY_QUERY,
// and their inverse variants.
type QueryResolver struct {
	queryRepo repository.QueryRepository
}

// NewQueryResolver creates a resolver backed by the given query repository.
func NewQueryResolver(queryRepo repository.QueryRepository) *QueryResolver {
	return &QueryResolver{queryRepo: queryRepo}
}

// ExpandedMenuItem holds a menu item plus optional query payload it was resolved with.
type ExpandedMenuItem struct {
	Menu         *models.UssdMenu
	QueryPayload map[string]any
}

// ResolveChildren takes raw children from the DB and expands/filters them based on query data.
// Returns the resolved list of expanded menu items. Items that should be hidden return nil.
func (qr *QueryResolver) ResolveChildren(ctx context.Context, children []*models.UssdMenu, msisdn, userID, botID, lang string) []ExpandedMenuItem {
	logger := util.Log(ctx)
	var result []ExpandedMenuItem

	for _, child := range children {
		if !models.IsQueryAction(child.Action) {
			result = append(result, ExpandedMenuItem{Menu: child})
			continue
		}

		expanded := qr.resolveQueryMenuItem(ctx, child, msisdn, userID, botID, lang)
		if expanded == nil {
			logger.WithField("menu_id", child.GetID()).Debug("menu item filtered by query resolution")
			continue
		}
		result = append(result, expanded...)
	}

	return result
}

func (qr *QueryResolver) resolveQueryMenuItem(ctx context.Context, menu *models.UssdMenu, msisdn, userID, _, lang string) []ExpandedMenuItem {
	logger := util.Log(ctx)
	queryName := menu.Extra
	if queryName == "" {
		return []ExpandedMenuItem{{Menu: menu}}
	}

	isInverse := models.IsInverseAction(menu.Action)

	switch menu.Action {
	case models.ActionChoiceFromManyQuery:
		return qr.resolveMany(ctx, menu, msisdn, userID, lang)

	case models.ActionChoiceFromQuery, models.ActionInverseChoiceFromQry:
		q, err := qr.queryRepo.FindByName(ctx, queryName, msisdn, userID)
		if err != nil {
			if isInverse {
				// No query found and inverse: show the item
				return []ExpandedMenuItem{{Menu: menu}}
			}
			logger.WithError(err).WithField("query_name", queryName).Debug("query not found")
			return nil
		}
		if isInverse {
			// Query found but inverse: hide the item
			return nil
		}
		expanded := applyQueryToMenu(menu, q.Payload)
		return []ExpandedMenuItem{{Menu: expanded, QueryPayload: q.Payload}}

	case models.ActionInputFromQuery, models.ActionInverseInputFromQry:
		q, err := qr.queryRepo.FindByName(ctx, queryName, msisdn, userID)
		if err != nil {
			if isInverse {
				return []ExpandedMenuItem{{Menu: menu}}
			}
			logger.WithError(err).WithField("query_name", queryName).Debug("query not found")
			return nil
		}
		if isInverse {
			return nil
		}
		expanded := applyQueryToMenu(menu, q.Payload)
		return []ExpandedMenuItem{{Menu: expanded, QueryPayload: q.Payload}}

	default:
		return []ExpandedMenuItem{{Menu: menu}}
	}
}

func (qr *QueryResolver) resolveMany(ctx context.Context, menu *models.UssdMenu, msisdn, userID, _ string) []ExpandedMenuItem {
	queryName := menu.Extra
	queries, err := qr.queryRepo.FindAllByName(ctx, queryName, msisdn, userID)
	if err != nil || len(queries) == 0 {
		return nil
	}

	var result []ExpandedMenuItem
	for _, q := range queries {
		expanded := applyQueryToMenu(menu, q.Payload)
		result = append(result, ExpandedMenuItem{Menu: expanded, QueryPayload: q.Payload})
	}
	return result
}

// applyQueryToMenu creates a copy of the menu with query payload values substituted
// into the name and message fields using {key} placeholder syntax.
func applyQueryToMenu(menu *models.UssdMenu, payload map[string]any) *models.UssdMenu {
	if payload == nil {
		return menu
	}

	// Create a shallow copy
	expanded := *menu
	expanded.Name = substitutePayload(menu.Name, payload)
	expanded.Message = substitutePayload(menu.Message, payload)
	return &expanded
}

// substitutePayload replaces {key} placeholders in the template with values from payload.
// Missing keys are left as-is.
func substitutePayload(template string, payload map[string]any) string {
	if !strings.Contains(template, "{") {
		return template
	}

	result := template
	for key, val := range payload {
		placeholder := fmt.Sprintf("{%s}", key)
		valStr := formatPayloadValue(val)
		result = strings.ReplaceAll(result, placeholder, valStr)
	}

	// Also try %(key)s style (Python format compatibility)
	for key, val := range payload {
		placeholder := fmt.Sprintf("%%(%s)s", key)
		valStr := formatPayloadValue(val)
		result = strings.ReplaceAll(result, placeholder, valStr)
	}

	return result
}

func formatPayloadValue(val any) string {
	switch v := val.(type) {
	case string:
		// Try to parse as JSON object for nested values
		var nested map[string]any
		if err := json.Unmarshal([]byte(v), &nested); err == nil {
			parts := make([]string, 0, len(nested))
			for _, nv := range nested {
				parts = append(parts, fmt.Sprintf("%v", nv))
			}
			return strings.Join(parts, " ")
		}
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
