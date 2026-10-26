package business_test

import (
	"testing"

	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/service-notification/apps/ussd/service/engine"
	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
)

const testAuthKey = "gateway-secret"

func (s *UssdTestSuite) TestGatewayRefusesAServiceWithoutAnAuthKey() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)
		res.Callers["+254700000011"] = engine.Caller{ProfileID: "profile-11", ContactID: "contact-11"}

		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionInformation,
			Name: "Root", Message: "Hello", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
			&models.UssdServiceConfig{ServiceID: "open", OwnerID: "seed", Name: models.ConfigInitialMenu, Value: root.GetID()}))

		// Anyone could post any number to an open gateway and act as that
		// caller, so a service without a key serves nobody.
		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			ServiceID: "open", MSISDN: "+254700000011", SessionExternal: "x1",
		})
		require.True(t, resp.IsEnd)
		require.Equal(t, "Authentication failed", resp.Message)
	})
}

func (s *UssdTestSuite) TestUnownedServiceIsNotClaimable() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		// Settings written before ownership was recorded have no owner.
		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
			&models.UssdServiceConfig{ServiceID: "legacy", Name: models.ConfigAuthKey, Value: "old"}))
		err := res.UssdBusiness.SetServiceConfig(ctx,
			&models.UssdServiceConfig{ServiceID: "legacy", OwnerID: "newcomer", Name: models.ConfigAuthKey, Value: "mine"})
		require.ErrorIs(t, err, repository.ErrServiceOwnedByAnother)
	})
}

func (s *UssdTestSuite) TestAfricasTalkingSessionReadsTheCallersOwnRows() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)
		res.Callers["+254700000009"] = engine.Caller{ProfileID: "profile-9", ContactID: "contact-9"}

		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionChoices,
			Validator: models.ValidatorChoice, Name: "Seed", Message: "Seed loans", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		// Items are listed highest order first.
		loan := &models.UssdMenu{OwnerID: "seed", ParentID: root.GetID(), Order: 2,
			Action: models.ActionChoiceFromQuery, Extra: "seed:loan", Name: "Loan balance {balance}",
			Message: "Balance {balance}", IsActive: true}
		loan.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, loan))
		info := &models.UssdMenu{OwnerID: "seed", ParentID: root.GetID(), Order: 1,
			Action: models.ActionInformation, Name: "Help", Message: "Call us", IsActive: true}
		info.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, info))

		for name, value := range map[string]string{
			models.ConfigInitialMenu:          root.GetID(),
			models.ConfigAuthKey:              testAuthKey,
			models.ConfigInputSeparator:       "*",
			models.ConfigDataBeginTemplate:    "CON %s",
			models.ConfigDataContinueTemplate: "CON %s",
			models.ConfigDataEndTemplate:      "END %s",
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
				&models.UssdServiceConfig{ServiceID: "seed-ussd", OwnerID: "seed", Name: name, Value: value}))
		}

		// Another service's row under the same name and contact is never read.
		require.NoError(t, res.UssdBusiness.ReplaceQueries(ctx,
			repository.QueryKey{OwnerID: "other", Name: "seed:loan", ContactID: "contact-9"},
			[]*models.UssdQuery{{Payload: data.JSONMap{"balance": "999"}}}))
		require.NoError(t, res.UssdBusiness.ReplaceQueries(ctx,
			repository.QueryKey{OwnerID: "seed", Name: "seed:loan", ContactID: "contact-9"},
			[]*models.UssdQuery{{Payload: data.JSONMap{"balance": "UGX 120,000"}}}))

		// Africa's Talking opens with an empty text, then resends the path.
		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			AuthKey:   testAuthKey,
			ServiceID: "seed-ussd", ServiceCode: "*384*1#", MSISDN: "254700000009",
			SessionExternal: "at-1", UserInput: "",
		})
		require.False(t, resp.IsEnd)
		require.Contains(t, resp.Message, "CON Seed loans")
		require.Contains(t, resp.Message, "Loan balance UGX 120,000")
		require.NotContains(t, resp.Message, "999")

		// The gateway formats the number differently on the next turn; the
		// session is the same one.
		resp = res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			AuthKey:   testAuthKey,
			ServiceID: "seed-ussd", MSISDN: "+254700000009", SessionExternal: "at-1", UserInput: "2",
		})
		require.True(t, resp.IsEnd)
		require.Equal(t, "END Call us", resp.Message)
	})
}

func (s *UssdTestSuite) TestCumulativeInputTakesTheLastSegment() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionChoices,
			Validator: models.ValidatorChoice, Name: "Root", Message: "Root", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		sub := &models.UssdMenu{OwnerID: "seed", ParentID: root.GetID(), Order: 1,
			Action: models.ActionChoices, Validator: models.ValidatorChoice, Name: "Loans",
			Message: "Loans", IsActive: true}
		sub.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, sub))
		leaf := &models.UssdMenu{OwnerID: "seed", ParentID: sub.GetID(), Order: 1,
			Action: models.ActionInformation, Name: "Rates", Message: "Rates are 4%", IsActive: true}
		leaf.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, leaf))
		for name, value := range map[string]string{
			models.ConfigInitialMenu:    root.GetID(),
			models.ConfigAuthKey:        testAuthKey,
			models.ConfigInputSeparator: "*",
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
				&models.UssdServiceConfig{ServiceID: "cum", OwnerID: "seed", Name: name, Value: value}))
		}

		for _, text := range []string{"", "1"} {
			resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
				AuthKey:   testAuthKey,
				ServiceID: "cum", MSISDN: "+254700000010", SessionExternal: "at-2", UserInput: text,
			})
			require.False(t, resp.IsEnd, text)
		}
		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{
			AuthKey:   testAuthKey,
			ServiceID: "cum", MSISDN: "+254700000010", SessionExternal: "at-2", UserInput: "1*1",
		})
		require.True(t, resp.IsEnd)
		require.Contains(t, resp.Message, "Rates are 4%")
	})
}

func (s *UssdTestSuite) TestServiceIsOwnedByItsFirstConfigurer() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)

		require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
			&models.UssdServiceConfig{ServiceID: "owned", OwnerID: "seed", Name: models.ConfigAuthKey, Value: "k1"}))
		err := res.UssdBusiness.SetServiceConfig(ctx,
			&models.UssdServiceConfig{ServiceID: "owned", OwnerID: "intruder", Name: models.ConfigAuthKey, Value: "k2"})
		require.ErrorIs(t, err, repository.ErrServiceOwnedByAnother)

		config, err := res.UssdBusiness.GetServiceConfig(ctx, "owned")
		require.NoError(t, err)
		require.Equal(t, "k1", config[models.ConfigAuthKey])

		owner, err := res.UssdBusiness.ServiceOwner(ctx, "owned")
		require.NoError(t, err)
		require.Equal(t, "seed", owner)
	})
}
