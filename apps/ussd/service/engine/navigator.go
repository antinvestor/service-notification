package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/util"
)

// Navigator processes a single USSD turn: reads the menu tree at the current position,
// validates user input, determines the next step, and builds the response display.
type Navigator struct {
	menuRepo        repository.MenuRepository
	translationRepo repository.TranslationRepository
	queryResolver   *QueryResolver
	validator       *InputValidator
}

// NewNavigator creates a navigator with the given dependencies.
func NewNavigator(menuRepo repository.MenuRepository, translationRepo repository.TranslationRepository, queryResolver *QueryResolver, validator *InputValidator) *Navigator {
	return &Navigator{
		menuRepo:        menuRepo,
		translationRepo: translationRepo,
		queryResolver:   queryResolver,
		validator:       validator,
	}
}

// NavigationContext holds the state needed for one USSD navigation turn.
type NavigationContext struct {
	ServiceID string
	UserID    string
	BotID     string
	MSISDN    string
	Lang      string
	MenuID    string
	UserInput string
}

// ProcessTurn handles one round of user interaction: validate input at current menu,
// determine the next menu, and return the result.
func (n *Navigator) ProcessTurn(ctx context.Context, nc NavigationContext) (*ProcessingResult, error) {
	logger := util.Log(ctx).WithFields(map[string]any{
		"menu_id":    nc.MenuID,
		"user_input": nc.UserInput,
		"msisdn":     maskMSISDN(nc.MSISDN),
	})

	// Load current menu and its children in a single query
	dbParent, dbChildren, err := n.menuRepo.GetWithChildren(ctx, nc.MenuID)
	if err != nil {
		logger.WithError(err).Error("failed to load menu tree")
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: "Service error"}, nil
	}

	if dbParent == nil {
		logger.Warn("menu not found, terminating")
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: "Menu not found"}, nil
	}

	// Work on shallow copies so DB-loaded originals are never mutated
	parent := shallowCopyMenu(dbParent)
	rawChildren := shallowCopyMenus(dbChildren)

	// Load translations for all menus in this turn
	n.applyTranslations(ctx, parent, rawChildren, nc.Lang)

	// Expand children through query resolution
	expandedChildren := n.queryResolver.ResolveChildren(ctx, rawChildren, nc.MSISDN, nc.UserID, nc.BotID, nc.Lang)

	// First turn at this menu (no user input yet) — display the menu
	if nc.UserInput == "" {
		return n.displayMenu(parent, expandedChildren, nc)
	}

	// Validate user input
	validationResult := n.validator.Validate(ctx, parent, nc.UserInput, len(expandedChildren), nc.Lang)
	if !validationResult.Valid {
		// Re-display current menu with error
		display := n.buildMenuDisplay(parent, expandedChildren, nc)
		display.Message = validationResult.ErrorMessage + "\n" + display.Message
		return &ProcessingResult{
			Outcome:    OutcomeContinue,
			Message:    appendNavHints(display.Message, parent.ParentID),
			NextMenuID: nc.MenuID,
			PrevMenuID: parent.ParentID,
		}, nil
	}

	// Input is valid — determine next step based on action type
	return n.processAction(ctx, nc, parent, expandedChildren)
}

// applyTranslations loads translations for the given language and overlays
// translated Name/Message/ErrorMessage onto shallow copies of the menu structs.
// The original GORM model pointers are not mutated.
func (n *Navigator) applyTranslations(ctx context.Context, parent *models.UssdMenu, children []*models.UssdMenu, lang string) {
	if lang == "" {
		return
	}

	// Collect all menu IDs
	menuIDs := make([]string, 0, 1+len(children))
	menuIDs = append(menuIDs, parent.GetID())
	for _, c := range children {
		menuIDs = append(menuIDs, c.GetID())
	}

	translations, err := n.translationRepo.GetByMenuIDs(ctx, menuIDs, lang)
	if err != nil || len(translations) == 0 {
		return
	}

	// Index by menu ID
	tMap := make(map[string]*models.UssdTranslation, len(translations))
	for _, t := range translations {
		tMap[t.MenuID] = t
	}

	// Apply to parent (shallow copy then overlay)
	if t, ok := tMap[parent.GetID()]; ok {
		applyTranslation(parent, t)
	}

	// Apply to children (shallow copy then overlay)
	for _, c := range children {
		if t, ok := tMap[c.GetID()]; ok {
			applyTranslation(c, t)
		}
	}
}

func applyTranslation(menu *models.UssdMenu, t *models.UssdTranslation) {
	if t.Name != "" {
		menu.Name = t.Name
	}
	if t.Message != "" {
		menu.Message = t.Message
	}
	if t.ErrorMessage != "" {
		menu.ErrorMessage = t.ErrorMessage
	}
}

// displayMenu renders the current menu for first-time display (no input yet).
func (n *Navigator) displayMenu(parent *models.UssdMenu, children []ExpandedMenuItem, nc NavigationContext) (*ProcessingResult, error) {
	display := n.buildMenuDisplay(parent, children, nc)
	return &ProcessingResult{
		Outcome:       OutcomeContinue,
		Message:       appendNavHints(display.Message, parent.ParentID),
		NextMenuID:    nc.MenuID,
		PrevMenuID:    parent.ParentID,
		CollectionKey: parent.CollectionKey,
	}, nil
}

// processAction dispatches to the correct action handler based on the parent menu's action type.
func (n *Navigator) processAction(ctx context.Context, nc NavigationContext, parent *models.UssdMenu, children []ExpandedMenuItem) (*ProcessingResult, error) {
	action := parent.Action

	// For choice-based actions, resolve which child was selected
	if models.IsChoiceAction(action) && isDigit(nc.UserInput) {
		idx, _ := strconv.Atoi(nc.UserInput)
		if idx >= 1 && idx <= len(children) {
			selected := children[idx-1]
			return n.advanceToChild(ctx, nc, parent, selected, children)
		}
	}

	// For input-based actions, advance to first child
	switch action {
	case models.ActionInput, models.ActionInputFromQuery, models.ActionInverseInputFromQry,
		models.ActionInputFromURL, models.ActionInputFromCode:
		if len(children) > 0 {
			return n.advanceToChild(ctx, nc, parent, children[0], children)
		}
		// Leaf input node — done collecting
		return &ProcessingResult{
			Outcome:       OutcomeDone,
			Message:       parent.Message,
			CollectionKey: parent.CollectionKey,
			CollectedData: nc.UserInput,
			IsPreference:  parent.IsPreference,
		}, nil

	case models.ActionChoicesValue:
		if len(children) > 0 {
			return &ProcessingResult{
				Outcome:       OutcomeDone,
				Message:       children[0].Menu.Message,
				CollectionKey: children[0].Menu.CollectionKey,
				CollectedData: nc.UserInput,
				IsPreference:  children[0].Menu.IsPreference,
			}, nil
		}
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: parent.Message}, nil

	case models.ActionChoicesValueContinue:
		if len(children) > 0 {
			return &ProcessingResult{
				Outcome:       OutcomeDoneContinue,
				Message:       children[0].Menu.Message,
				CollectionKey: children[0].Menu.CollectionKey,
				CollectedData: nc.UserInput,
				IsPreference:  children[0].Menu.IsPreference,
			}, nil
		}
		return &ProcessingResult{Outcome: OutcomeDoneContinue, Message: parent.Message}, nil

	case models.ActionChoicesCancel:
		return &ProcessingResult{Outcome: OutcomeCanceled, Message: parent.Message}, nil

	case models.ActionInformation:
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: parent.Message}, nil

	default:
		// For choices with valid input already handled above
		if len(children) > 0 {
			return n.advanceToChild(ctx, nc, parent, children[0], children)
		}
		return &ProcessingResult{
			Outcome:       OutcomeDone,
			Message:       parent.Message,
			CollectionKey: parent.CollectionKey,
			CollectedData: nc.UserInput,
			IsPreference:  parent.IsPreference,
		}, nil
	}
}

// advanceToChild moves to the selected child menu and renders its content.
func (n *Navigator) advanceToChild(ctx context.Context, nc NavigationContext, parent *models.UssdMenu, selected ExpandedMenuItem, _ []ExpandedMenuItem) (*ProcessingResult, error) {
	child := selected.Menu

	// Collect data from the current step
	collectedKey := parent.CollectionKey
	collectedData := nc.UserInput
	if selected.QueryPayload != nil {
		// For query-based choices, the collected data comes from the query payload
		if v, ok := selected.QueryPayload[child.CollectionKey]; ok {
			collectedData = fmt.Sprintf("%v", v)
		}
	}

	// Check if the child is a terminal action
	switch child.Action {
	case models.ActionChoicesValue:
		return &ProcessingResult{
			Outcome:       OutcomeDone,
			Message:       child.Message,
			CollectionKey: collectedKey,
			CollectedData: collectedData,
			IsPreference:  child.IsPreference,
		}, nil

	case models.ActionChoicesValueContinue:
		return &ProcessingResult{
			Outcome:       OutcomeDoneContinue,
			Message:       child.Message,
			CollectionKey: collectedKey,
			CollectedData: collectedData,
			IsPreference:  child.IsPreference,
		}, nil

	case models.ActionChoicesCancel:
		return &ProcessingResult{Outcome: OutcomeCanceled, Message: child.Message}, nil

	case models.ActionInformation:
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: child.Message}, nil
	}

	// Load grandchildren to render the next screen
	_, dbGrandchildren, err := n.menuRepo.GetWithChildren(ctx, child.GetID())
	if err != nil {
		return &ProcessingResult{Outcome: OutcomeTerminate, Message: "Service error"}, nil
	}

	grandchildren := shallowCopyMenus(dbGrandchildren)
	n.applyTranslations(ctx, child, grandchildren, nc.Lang)

	expandedGrandchildren := n.queryResolver.ResolveChildren(ctx, grandchildren, nc.MSISDN, nc.UserID, nc.BotID, nc.Lang)

	if len(expandedGrandchildren) == 0 {
		// Leaf node — display its message as input prompt or done
		if child.Action == models.ActionInput || child.Action == models.ActionInputFromURL ||
			child.Action == models.ActionInputFromCode || child.Action == models.ActionInputFromQuery ||
			child.Action == models.ActionInverseInputFromQry {
			return &ProcessingResult{
				Outcome:       OutcomeContinue,
				Message:       appendNavHints(child.Message, parent.GetID()),
				NextMenuID:    child.GetID(),
				PrevMenuID:    parent.GetID(),
				CollectionKey: collectedKey,
				CollectedData: collectedData,
				IsPreference:  parent.IsPreference,
			}, nil
		}
		// Terminal leaf
		return &ProcessingResult{
			Outcome:       OutcomeDone,
			Message:       child.Message,
			CollectionKey: collectedKey,
			CollectedData: collectedData,
			IsPreference:  child.IsPreference,
		}, nil
	}

	// Child has grandchildren — render it as a choices menu
	childNC := NavigationContext{
		ServiceID: nc.ServiceID,
		UserID:    nc.UserID,
		BotID:     nc.BotID,
		MSISDN:    nc.MSISDN,
		Lang:      nc.Lang,
		MenuID:    child.GetID(),
	}
	display := n.buildMenuDisplay(child, expandedGrandchildren, childNC)

	return &ProcessingResult{
		Outcome:       OutcomeContinue,
		Message:       appendNavHints(display.Message, parent.GetID()),
		NextMenuID:    child.GetID(),
		PrevMenuID:    parent.GetID(),
		CollectionKey: collectedKey,
		CollectedData: collectedData,
		IsPreference:  parent.IsPreference,
	}, nil
}

// buildMenuDisplay renders a menu and its children into a displayable string.
func (n *Navigator) buildMenuDisplay(parent *models.UssdMenu, children []ExpandedMenuItem, _ NavigationContext) MenuDisplay {
	if models.IsChoiceAction(parent.Action) || len(children) > 0 {
		return buildChoicesDisplay(parent, children)
	}

	// Input or information menu — just show the message
	return MenuDisplay{
		Message:       parent.Message,
		Validator:     parent.Validator,
		ErrorMessage:  parent.ErrorMessage,
		NextMenuID:    parent.GetID(),
		CollectionKey: parent.CollectionKey,
		IsPreference:  parent.IsPreference,
	}
}

// buildChoicesDisplay creates a numbered list of choices.
func buildChoicesDisplay(parent *models.UssdMenu, children []ExpandedMenuItem) MenuDisplay {
	var sb strings.Builder
	sb.WriteString(parent.Message)
	sb.WriteByte('\n')

	for i, child := range children {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, child.Menu.Name)
	}

	return MenuDisplay{
		Message:       strings.TrimRight(sb.String(), "\n"),
		Validator:     models.ValidatorChoice,
		NextMenuID:    parent.GetID(),
		CollectionKey: parent.CollectionKey,
		IsPreference:  parent.IsPreference,
	}
}

func shallowCopyMenu(m *models.UssdMenu) *models.UssdMenu {
	cp := *m
	return &cp
}

func shallowCopyMenus(menus []*models.UssdMenu) []*models.UssdMenu {
	if menus == nil {
		return nil
	}
	copies := make([]*models.UssdMenu, len(menus))
	for i, m := range menus {
		copies[i] = shallowCopyMenu(m)
	}
	return copies
}

// appendNavHints adds "0. Back" / "00. Main menu" hints to the message.
func appendNavHints(message, parentID string) string {
	if parentID == "" {
		return message
	}
	return message + "\n0. Back\n00. Main menu"
}
