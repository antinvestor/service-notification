// Package credentials resolves per-connection provider credentials for integration
// workers from the message headers the notification service attaches when it queues
// an outbound notification.
package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/antinvestor/settingz/protocolbuffers/go/settings/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/pkg/constants"
)

// ErrNoConnection is returned when neither a connection nor a route id header is present.
var ErrNoConnection = errors.New("no connection credentials or route id header on message")

// DefaultCacheTTL bounds how long a resolved credential set is reused before the
// settings service is consulted again, so rotated secrets pick up without a restart.
const DefaultCacheTTL = 5 * time.Minute

// Resolver looks up credential maps stored as JSON in the settings service.
//
// The setting is addressed by connection name: the X-API_CONNECTION_CREDENTIALS header
// when present, otherwise the route id the notification was queued on. Both are
// attached by the notification service's out-queue step, so a route row plus one
// setting is enough to enable a provider for a partition.
type Resolver struct {
	settingsCli settingsv1connect.SettingsServiceClient
	object      string
	objectID    string
	ttl         time.Duration

	mu    sync.Mutex
	cache map[string]cachedEntry
}

type cachedEntry struct {
	values    map[string]string
	expiresAt time.Time
}

// New creates a resolver for one integration's settings namespace.
func New(settingsCli settingsv1connect.SettingsServiceClient, integrationName, integrationID string) *Resolver {
	return &Resolver{
		settingsCli: settingsCli,
		object:      integrationName,
		objectID:    integrationID,
		ttl:         DefaultCacheTTL,
		cache:       map[string]cachedEntry{},
	}
}

// WithTTL overrides the cache lifetime; a zero or negative ttl disables caching.
func (r *Resolver) WithTTL(ttl time.Duration) *Resolver {
	r.ttl = ttl
	return r
}

// ConnectionName picks the settings key for a message from its headers.
func ConnectionName(headers map[string]string) string {
	if name := headers[constants.APIConnectionCredentialsHeaderName]; name != "" {
		return name
	}
	return headers[constants.RouteIDHeaderName]
}

// Resolve returns the credential map for the connection identified by the headers.
func (r *Resolver) Resolve(ctx context.Context, headers map[string]string) (map[string]string, error) {
	name := ConnectionName(headers)
	if name == "" {
		return nil, ErrNoConnection
	}
	return r.ResolveByName(ctx, name)
}

// ResolveByName returns the credential map stored under the given connection name.
func (r *Resolver) ResolveByName(ctx context.Context, name string) (map[string]string, error) {
	if values, ok := r.cached(name); ok {
		return values, nil
	}

	resp, err := r.settingsCli.Get(ctx, connect.NewRequest(&settingsv1.GetRequest{
		Key: &settingsv1.Setting{
			Name:     name,
			Object:   r.object,
			ObjectId: r.objectID,
			Module:   r.object,
		},
	}))
	if err != nil {
		return nil, fmt.Errorf("credentials for connection %q: %w", name, err)
	}

	raw := resp.Msg.GetData().GetValue()
	if raw == "" {
		return nil, fmt.Errorf("credentials for connection %q: setting has no value", name)
	}

	var values map[string]string
	if err = json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("credentials for connection %q: value is not a JSON object of strings: %w", name, err)
	}

	r.store(name, values)
	return values, nil
}

// Forget drops a cached connection, for example after the provider rejected its token.
func (r *Resolver) Forget(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, name)
}

func (r *Resolver) cached(name string) (map[string]string, bool) {
	if r.ttl <= 0 {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[name]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.values, true
}

func (r *Resolver) store(name string, values map[string]string) {
	if r.ttl <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[name] = cachedEntry{values: values, expiresAt: time.Now().Add(r.ttl)}
}
