package events

import (
	"context"
	"errors"
	"fmt"
	"slices"

	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
)

// ErrNoRouteMatched is returned when no route exists for any of the candidate channels.
var ErrNoRouteMatched = errors.New("no routes matched for notification")

// routeLookup is the subset of the route repository needed for channel routing.
type routeLookup interface {
	GetByID(ctx context.Context, id string) (*models.Route, error)
	GetByModeTypeAndPartitionID(ctx context.Context, mode string, routeType string, partitionID string) ([]*models.Route, error)
}

// channelTypesForContact returns the delivery channels a contact type can be reached on,
// in order of preference. Phone contacts prefer WhatsApp and fall back to SMS.
func channelTypesForContact(contactType profilev1.ContactType) []string {
	switch contactType {
	case profilev1.ContactType_MSISDN:
		return []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm}
	case profilev1.ContactType_EMAIL:
		return []string{models.RouteTypeEmailForm}
	default:
		return []string{models.RouteTypeAny}
	}
}

// routeWithChannelPreference resolves the route for a notification.
//
//  1. An explicit RouteID always wins; the channel is the notification's type, or the route's type.
//  2. An explicit NotificationType that is one of channelTypes restricts routing to that channel.
//  3. Otherwise channels are tried in order and the first with a route wins; fallback reports
//     whether a channel after the first was used.
//
// On success the notification's type is set to the chosen channel.
func routeWithChannelPreference(ctx context.Context, repo routeLookup, mode string,
	n *models.Notification, channelTypes []string) (route *models.Route, channel string, fallback bool, err error) {

	if n.RouteID != "" {
		route, err = repo.GetByID(ctx, n.RouteID)
		if err != nil {
			return nil, "", false, err
		}
		channel = n.NotificationType
		if channel == "" {
			channel = route.RouteType
		}
		n.NotificationType = channel
		return route, channel, false, nil
	}

	if len(channelTypes) == 0 {
		channelTypes = []string{models.RouteTypeAny}
	}

	if n.NotificationType != "" && slices.Contains(channelTypes, n.NotificationType) {
		channelTypes = []string{n.NotificationType}
	}

	for i, candidate := range channelTypes {
		routes, lookupErr := repo.GetByModeTypeAndPartitionID(ctx, mode, candidate, n.PartitionID)
		if lookupErr != nil {
			return nil, "", false, lookupErr
		}
		if len(routes) == 0 {
			continue
		}

		n.NotificationType = candidate
		return routes[0], candidate, i > 0, nil
	}

	return nil, "", false, fmt.Errorf("%w: %s (channels %v)", ErrNoRouteMatched, n.GetID(), channelTypes)
}
