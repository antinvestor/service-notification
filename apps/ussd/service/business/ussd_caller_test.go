package business_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"

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

func (s *UssdTestSuite) TestSecretInputIsSealedForTheService() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)
		res.Callers["+254700000012"] = engine.Caller{ProfileID: "profile-12", ContactID: "contact-12"}

		public, private, err := box.GenerateKey(rand.Reader)
		require.NoError(t, err)

		// Root -> "Borrow" input (amount) -> PIN input (secret) -> done.
		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionChoices,
			Validator: models.ValidatorChoice, Name: "Seed", Message: "Seed", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		amount := &models.UssdMenu{OwnerID: "seed", ParentID: root.GetID(), Action: models.ActionInput,
			Validator: models.ValidatorNumber, Name: "Borrow", Message: "Enter amount", CollectionKey: "amount",
			IsActive: true}
		amount.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, amount))
		pin := &models.UssdMenu{OwnerID: "seed", ParentID: amount.GetID(), Action: models.ActionInput,
			Validator: models.ValidatorNumber, Name: "PIN", Message: "Enter your PIN", CollectionKey: "pin",
			IsSecret: true, IsActive: true}
		pin.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, pin))
		confirm := &models.UssdMenu{OwnerID: "seed", ParentID: pin.GetID(), Action: models.ActionInput,
			Validator: models.ValidatorNumber, Name: "Confirm", Message: "Press 1 to confirm", CollectionKey: "confirm",
			IsActive: true}
		confirm.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, confirm))
		done := &models.UssdMenu{OwnerID: "seed", ParentID: confirm.GetID(), Action: models.ActionChoicesValue,
			Name: "Done", Message: "Request received", IsActive: true}
		done.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, done))

		for name, value := range map[string]string{
			models.ConfigInitialMenu:  root.GetID(),
			models.ConfigAuthKey:      testAuthKey,
			models.ConfigSecretBoxKey: base64.StdEncoding.EncodeToString(public[:]),
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
				&models.UssdServiceConfig{ServiceID: "sealed", OwnerID: "seed", Name: name, Value: value}))
		}

		turn := func(input string) engine.USSDResponse {
			return res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{AuthKey: testAuthKey,
				ServiceID: "sealed", MSISDN: "+254700000012", SessionExternal: "sx", UserInput: input})
		}
		require.Contains(t, turn("").Message, "1. Borrow")
		// An input with a continuation shows only its prompt.
		prompt := turn("1")
		require.Contains(t, prompt.Message, "Enter amount")
		require.NotContains(t, prompt.Message, "1. PIN")
		require.Contains(t, turn("5000").Message, "Enter your PIN")
		require.Contains(t, turn("4321").Message, "Press 1 to confirm")

		// The PIN is kept only sealed; the amount, not secret, in the clear.
		session, err := res.SessionRepo.FindActiveSession(ctx, "+254700000012", "sealed", "sx")
		require.NoError(t, err)
		require.Equal(t, "5000", session.CollectedData.GetString("amount"))
		stored := session.CollectedData.GetString("pin")
		require.True(t, strings.HasPrefix(stored, engine.SealedPrefix), stored)
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, engine.SealedPrefix))
		require.NoError(t, err)
		opened, ok := box.OpenAnonymous(nil, raw, public, private)
		require.True(t, ok)
		require.Equal(t, "4321", string(opened))

		final := turn("1")
		require.True(t, final.IsEnd)
		require.Equal(t, "Request received", final.Message)
	})
}

func TestSealedSecretOpensOnlyWithTheServiceKey(t *testing.T) {
	public, private, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	sealed, err := engine.SealSecret(base64.StdEncoding.EncodeToString(public[:]), "4321")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(sealed, engine.SealedPrefix))
	require.NotContains(t, sealed, "4321")

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, engine.SealedPrefix))
	require.NoError(t, err)
	opened, ok := box.OpenAnonymous(nil, raw, public, private)
	require.True(t, ok)
	require.Equal(t, "4321", string(opened))

	_, err = engine.SealSecret("", "4321")
	require.ErrorIs(t, err, engine.ErrNoSecretKey)
}

func (s *UssdTestSuite) TestSessionStartPullsTheCallersCurrentRows() {
	var asked engine.StateRequest
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&asked)
		_ = json.NewEncoder(w).Encode(engine.StateResponse{Rows: map[string][]map[string]any{
			"seed:loan": {{"balance": "KES 750"}},
		}})
	}))
	defer srv.Close()
	s.ServiceClient = srv.Client()
	s.TrustedHosts = []string{strings.TrimPrefix(srv.URL, "https://")}
	defer func() { s.ServiceClient, s.TrustedHosts = nil, nil }()

	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)
		res.Callers["+254700000013"] = engine.Caller{ProfileID: "profile-13", ContactID: "contact-13"}

		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionChoices,
			Validator: models.ValidatorChoice, Name: "Seed", Message: "Seed", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		loan := &models.UssdMenu{OwnerID: "seed", ParentID: root.GetID(), Action: models.ActionChoiceFromQuery,
			Extra: "seed:loan", Name: "Balance {balance}", Message: "Balance {balance}", IsActive: true}
		loan.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, loan))

		// A stale row from before this session is replaced.
		require.NoError(t, res.UssdBusiness.ReplaceQueries(ctx,
			repository.QueryKey{OwnerID: "seed", Name: "seed:loan", ContactID: "contact-13"},
			[]*models.UssdQuery{{Payload: data.JSONMap{"balance": "KES 1,000"}}}))

		for name, value := range map[string]string{
			models.ConfigInitialMenu:     root.GetID(),
			models.ConfigAuthKey:         testAuthKey,
			models.ConfigSessionStartURL: srv.URL + "/state",
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
				&models.UssdServiceConfig{ServiceID: "pull", OwnerID: "seed", Name: name, Value: value}))
		}

		resp := res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{AuthKey: testAuthKey,
			ServiceID: "pull", MSISDN: "+254700000013", SessionExternal: "p1"})
		require.Contains(t, resp.Message, "Balance KES 750")
		// The pulled set replaced the stale one rather than adding to it.
		active, err := res.QueryRepo.FindAllByName(ctx,
			repository.QueryKey{OwnerID: "seed", Name: "seed:loan", ContactID: "contact-13"})
		require.NoError(t, err)
		require.Len(t, active, 1)
		require.NotContains(t, resp.Message, "KES 1,000")
		require.Equal(t, engine.StateRequest{ServiceID: "pull", ProfileID: "profile-13", ContactID: "contact-13"}, asked)
	})
}

func (s *UssdTestSuite) TestSessionStartURLMustBeATrustedHost() {
	called := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"rows":{}}`))
	}))
	defer srv.Close()
	s.ServiceClient = srv.Client()
	defer func() { s.ServiceClient = nil }()

	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, res := s.CreateService(t, dep)
		res.Callers["+254700000014"] = engine.Caller{ProfileID: "profile-14", ContactID: "contact-14"}
		root := &models.UssdMenu{OwnerID: "seed", Action: models.ActionInformation,
			Name: "Root", Message: "Hello", IsActive: true}
		root.GenID(ctx)
		require.NoError(t, res.UssdBusiness.CreateMenu(ctx, root))
		for name, value := range map[string]string{
			models.ConfigInitialMenu:     root.GetID(),
			models.ConfigAuthKey:         testAuthKey,
			models.ConfigSessionStartURL: srv.URL + "/steal-token",
		} {
			require.NoError(t, res.UssdBusiness.SetServiceConfig(ctx,
				&models.UssdServiceConfig{ServiceID: "untrusted", OwnerID: "seed", Name: name, Value: value}))
		}

		res.UssdBusiness.ProcessUSSD(ctx, engine.USSDRequest{AuthKey: testAuthKey,
			ServiceID: "untrusted", MSISDN: "+254700000014", SessionExternal: "u1"})
		require.False(t, called, "the engine's token must not go to an untrusted host")
	})
}
