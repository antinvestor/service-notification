package business_test

import (
	"testing"

	"github.com/antinvestor/service-notification/apps/ussd/service/engine"
	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/tests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UssdTestSuite struct {
	tests.BaseTestSuite
}

func TestUssdSuite(t *testing.T) {
	suite.Run(t, new(UssdTestSuite))
}

func (s *UssdTestSuite) TestCreateAndGetMenu() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		menu := &models.UssdMenu{
			OwnerID:  "test-owner",
			Action:   models.ActionChoices,
			Name:     "Main Menu",
			Message:  "Welcome. Select an option:",
			IsActive: true,
		}
		menu.GenID(ctx)

		err := res.UssdBusiness.CreateMenu(ctx, menu)
		require.NoError(t, err)

		fetched, err := res.UssdBusiness.GetMenu(ctx, menu.GetID())
		require.NoError(t, err)
		require.Equal(t, "Main Menu", fetched.Name)
		require.Equal(t, models.ActionChoices, fetched.Action)
	})
}

func (s *UssdTestSuite) TestMenuTree() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		// Create root menu
		root := &models.UssdMenu{
			OwnerID:   "test-owner",
			Action:    models.ActionChoices,
			Validator: models.ValidatorChoice,
			Name:      "Main Menu",
			Message:   "Welcome. Choose:",
			IsActive:  true,
		}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))

		// Create children
		child1 := &models.UssdMenu{
			OwnerID:       "test-owner",
			ParentID:      root.GetID(),
			Action:        models.ActionInput,
			Validator:     models.ValidatorInput,
			Name:          "Check Balance",
			Message:       "Enter account number:",
			CollectionKey: "account_number",
			IsActive:      true,
		}
		child1.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, child1))

		child2 := &models.UssdMenu{
			OwnerID:  "test-owner",
			ParentID: root.GetID(),
			Action:   models.ActionInformation,
			Name:     "About",
			Message:  "Service v1.0",
			IsActive: true,
		}
		child2.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, child2))

		// Verify children
		children, err := res.UssdBusiness.GetMenuChildren(ctx, root.GetID())
		require.NoError(t, err)
		require.Len(t, children, 2)
	})
}

func (s *UssdTestSuite) TestTranslation() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		menu := &models.UssdMenu{
			OwnerID:  "test-owner",
			Action:   models.ActionChoices,
			Name:     "Main Menu",
			Message:  "Welcome",
			IsActive: true,
		}
		menu.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, menu))

		// Set French translation
		translation := &models.UssdTranslation{
			MenuID:  menu.GetID(),
			Code:    "fr",
			Name:    "Menu Principal",
			Message: "Bienvenue",
		}
		require.NoError(t, res.UssdBusiness.SetTranslation(ctx, translation))

		// Retrieve it
		fetched, err := res.UssdBusiness.GetTranslations(ctx, menu.GetID(), "fr")
		require.NoError(t, err)
		require.Equal(t, "Menu Principal", fetched.Name)
		require.Equal(t, "Bienvenue", fetched.Message)
	})
}

func (s *UssdTestSuite) TestDuplicateMenu() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		// Create a menu tree
		root := &models.UssdMenu{
			OwnerID:  "original-owner",
			Action:   models.ActionChoices,
			Name:     "Root",
			Message:  "Select:",
			IsActive: true,
		}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))

		child := &models.UssdMenu{
			OwnerID:  "original-owner",
			ParentID: root.GetID(),
			Action:   models.ActionInput,
			Name:     "Child",
			Message:  "Enter:",
			IsActive: true,
		}
		child.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, child))

		// Duplicate
		newRoot, err := res.UssdBusiness.DuplicateMenu(ctx, root.GetID(), "new-owner", "")
		require.NoError(t, err)
		require.NotEqual(t, root.GetID(), newRoot.GetID())
		require.Equal(t, "new-owner", newRoot.OwnerID)

		// Check duplicated children
		newChildren, err := res.UssdBusiness.GetMenuChildren(ctx, newRoot.GetID())
		require.NoError(t, err)
		require.Len(t, newChildren, 1)
		require.Equal(t, "Child", newChildren[0].Name)
	})
}

func (s *UssdTestSuite) TestServiceConfig() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		cfg := &models.UssdServiceConfig{
			ServiceID: "svc-001",
			Name:      models.ConfigInitialMenu,
			Value:     "menu-root-123",
		}
		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx, cfg))

		cfg2 := &models.UssdServiceConfig{
			ServiceID: "svc-001",
			Name:      models.ConfigDataField,
			Value:     "input",
		}
		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx, cfg2))

		config, err := res.UssdBusiness.GetServiceConfig(ctx, "svc-001")
		require.NoError(t, err)
		require.Equal(t, "menu-root-123", config[models.ConfigInitialMenu])
		require.Equal(t, "input", config[models.ConfigDataField])
	})
}

func (s *UssdTestSuite) TestEndToEndUSSDFlow() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		// 1. Create menu tree: Root -> [Balance, Transfer]
		root := &models.UssdMenu{
			OwnerID:   "test-owner",
			Action:    models.ActionChoices,
			Validator: models.ValidatorChoice,
			Name:      "Main Menu",
			Message:   "Welcome to Test Bank",
			IsActive:  true,
		}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))

		balance := &models.UssdMenu{
			OwnerID:       "test-owner",
			ParentID:      root.GetID(),
			Order:         2,
			Action:        models.ActionInput,
			Validator:     models.ValidatorNumber,
			Name:          "Check Balance",
			Message:       "Enter your account number:",
			CollectionKey: "account_number",
			IsActive:      true,
		}
		balance.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, balance))

		transfer := &models.UssdMenu{
			OwnerID:  "test-owner",
			ParentID: root.GetID(),
			Order:    1,
			Action:   models.ActionInformation,
			Name:     "Transfer",
			Message:  "Transfer service coming soon",
			IsActive: true,
		}
		transfer.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, transfer))

		// 2. Configure the service
		for _, cfg := range []*models.UssdServiceConfig{
			{ServiceID: "ussd-001", Name: models.ConfigInitialMenu, Value: root.GetID()},
			{ServiceID: "ussd-001", Name: models.ConfigDataField, Value: "input"},
			{ServiceID: "ussd-001", Name: models.ConfigMSISDNField, Value: "msisdn"},
			{ServiceID: "ussd-001", Name: models.ConfigSessionField, Value: "sessionId"},
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx, cfg))
		}

		// 3. First request — should show root menu
		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID:       "ussd-001",
			MSISDN:          "+254700000001",
			SessionExternal: "sess-001",
			UserInput:       "",
		})
		require.False(t, resp.IsEnd, "first request should continue")
		require.Contains(t, resp.Message, "Welcome to Test Bank")
		require.Contains(t, resp.Message, "1.")
		require.Contains(t, resp.Message, "2.")

		// 4. Select option 2 (Transfer - INFORMATION action = terminal)
		resp = res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID:       "ussd-001",
			MSISDN:          "+254700000001",
			SessionExternal: "sess-002",
			UserInput:       "2",
		})
		// This should show Transfer as an info screen that terminates
		require.Contains(t, resp.Message, "Transfer service coming soon")
		require.True(t, resp.IsEnd)
	})
}

func (s *UssdTestSuite) TestNavigationBack() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		// Create menu tree
		root := &models.UssdMenu{
			OwnerID:   "owner",
			Action:    models.ActionChoices,
			Validator: models.ValidatorChoice,
			Name:      "Root",
			Message:   "Main:",
			IsActive:  true,
		}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))

		child := &models.UssdMenu{
			OwnerID:   "owner",
			ParentID:  root.GetID(),
			Action:    models.ActionChoices,
			Validator: models.ValidatorChoice,
			Name:      "Sub",
			Message:   "Sub menu:",
			IsActive:  true,
		}
		child.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, child))

		grandchild := &models.UssdMenu{
			OwnerID:  "owner",
			ParentID: child.GetID(),
			Action:   models.ActionInformation,
			Name:     "Info",
			Message:  "Some info",
			IsActive: true,
		}
		grandchild.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, grandchild))

		// Configure
		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx, &models.UssdServiceConfig{
			ServiceID: "nav-001", Name: models.ConfigInitialMenu, Value: root.GetID(),
		}))

		// First request — root menu
		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID: "nav-001", MSISDN: "+254", SessionExternal: "s1",
		})
		require.Contains(t, resp.Message, "Main:")

		// Select option 1 — go to sub menu
		resp = res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID: "nav-001", MSISDN: "+254", SessionExternal: "s1", UserInput: "1",
		})
		require.Contains(t, resp.Message, "Sub menu:")

		// Go back with "0"
		resp = res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID: "nav-001", MSISDN: "+254", SessionExternal: "s1", UserInput: "0",
		})
		require.Contains(t, resp.Message, "Main:")
	})
}

func (s *UssdTestSuite) TestSessionExpiry() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		cleaned, err := res.UssdBusiness.CleanupExpiredSessions(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, cleaned, int64(0))
	})
}

func (s *UssdTestSuite) TestDeleteMenu() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		root := &models.UssdMenu{
			OwnerID:  "owner",
			Action:   models.ActionChoices,
			Name:     "To Delete",
			Message:  "Will be deleted",
			IsActive: true,
		}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))

		child := &models.UssdMenu{
			OwnerID:  "owner",
			ParentID: root.GetID(),
			Action:   models.ActionInput,
			Name:     "Child",
			Message:  "Also deleted",
			IsActive: true,
		}
		child.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, child))

		// Delete cascading
		err := res.UssdBusiness.DeleteMenu(ctx, root.GetID())
		require.NoError(t, err)

		// Verify both are gone
		_, err = res.UssdBusiness.GetMenu(ctx, root.GetID())
		require.Error(t, err)

		_, err = res.UssdBusiness.GetMenu(ctx, child.GetID())
		require.Error(t, err)
	})
}
