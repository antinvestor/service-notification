package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/util"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
)

// maxStateResponse bounds what a service may return for one caller.
const maxStateResponse = 64 * 1024

// StateRequest is what the engine sends a service's session_start_url when
// a known caller opens a session.
type StateRequest struct {
	ServiceID string `json:"service_id"`
	ProfileID string `json:"profile_id"`
	ContactID string `json:"contact_id"`
}

// StateResponse is the caller's current data: for each row name, the rows
// the service's menus show this caller. A name left out keeps its rows; a
// name with no rows clears them.
type StateResponse struct {
	Rows map[string][]map[string]any `json:"rows"`
}

// statePuller fetches a caller's data from the owning service at the start
// of a session, so the first screen shows the caller's state as it is now.
type statePuller struct {
	client    *http.Client
	queryRepo repository.QueryRepository
	owners    repository.ServiceConfigRepository
	// trustedHosts are the only hosts a session_start_url may name: the call
	// carries this service's own token, which must never reach anyone else.
	trustedHosts []string
}

func (s *statePuller) trusted(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return false
	}
	return slices.Contains(s.trustedHosts, u.Host)
}

// pull asks the service for the caller's rows and stores them under the
// service's owner. A failure is logged and the session continues on the
// rows already stored.
func (s *statePuller) pull(ctx context.Context, rawURL, serviceID string, session *models.UssdSession) {
	if s == nil || s.client == nil || rawURL == "" {
		return
	}
	contactID := session.CollectedData.GetString(models.SessionKeyContactID)
	profileID := session.CollectedData.GetString(models.SessionKeyProfileID)
	if contactID == "" || profileID == "" {
		return
	}
	logger := util.Log(ctx).WithField("service_id", serviceID)
	if !s.trusted(rawURL) {
		logger.Warn("session_start_url is not an https URL on a trusted host; caller state not pulled")
		return
	}
	owner, err := s.owners.OwnerOf(ctx, serviceID)
	if err != nil || owner == "" {
		logger.WithError(err).Warn("service has no owner; caller state not pulled")
		return
	}

	state, err := s.fetch(ctx, rawURL, StateRequest{ServiceID: serviceID, ProfileID: profileID, ContactID: contactID})
	if err != nil {
		logger.WithError(err).Warn("caller state not pulled; using stored rows")
		return
	}
	for name, payloads := range state.Rows {
		rows := make([]*models.UssdQuery, 0, len(payloads))
		for _, payload := range payloads {
			row := &models.UssdQuery{
				OwnerID: owner, ContactID: contactID, ProfileID: profileID, Name: name,
				Payload: data.JSONMap(payload), IsMany: len(payloads) > 1, IsActive: true,
			}
			row.GenID(ctx)
			rows = append(rows, row)
		}
		key := repository.QueryKey{OwnerID: owner, Name: name, ContactID: contactID}
		if err = s.queryRepo.Replace(ctx, key, rows); err != nil {
			logger.WithError(err).WithField("row", name).Warn("caller state row not stored")
		}
	}
}

func (s *statePuller) fetch(ctx context.Context, rawURL string, in StateRequest) (*StateResponse, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session_start_url answered %s", resp.Status)
	}
	out := &StateResponse{}
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxStateResponse)).Decode(out); err != nil {
		return nil, fmt.Errorf("decode caller state: %w", err)
	}
	return out, nil
}
