package events

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/v3"
	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/frame/v3/events"
	"github.com/pitabwire/util"
)

// NotificationOutRouteEvent is the event name for routing outgoing notifications
const NotificationOutRouteEvent = "notification.out.route"

type NotificationOutRoute struct {
	eventMan events.Manager

	profileCli       profilev1connect.ProfileServiceClient
	notificationRepo repository.NotificationRepository
	routeRepo        repository.RouteRepository
}

// NewNotificationOutRoute creates a new NotificationOutRoute event handler
func NewNotificationOutRoute(ctx context.Context, eventMan events.Manager, profileCli profilev1connect.ProfileServiceClient, notificationRepo repository.NotificationRepository, routeRepo repository.RouteRepository) *NotificationOutRoute {

	return &NotificationOutRoute{
		eventMan:         eventMan,
		profileCli:       profileCli,
		notificationRepo: notificationRepo,
		routeRepo:        routeRepo,
	}
}

func (event *NotificationOutRoute) Name() string {
	return NotificationOutRouteEvent
}

func (event *NotificationOutRoute) PayloadType() any {
	pType := ""
	return &pType
}

func (event *NotificationOutRoute) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutRoute) Execute(ctx context.Context, payload any) error {

	notificationId := *payload.(*string)

	logger := util.Log(ctx).WithFields(map[string]any{"type": event.Name(), "notification_id": notificationId})
	defer logger.Release()
	logger.Debug("event handler started")

	n, err := event.notificationRepo.GetByID(ctx, notificationId)
	if err != nil {
		logger.WithError(err).Error("could not get notification from db")
		return err
	}

	var profileObj *profilev1.ProfileObject

	if n.RecipientProfileID == "" {

		if n.RecipientContactID == "" {
			logger.Error("recipient contact id is empty")

			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": "recipient contact id is empty",
					"step":  "validate_recipient",
				},
			}

			nStatus.GenID(ctx)
			_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			return nil
		}

		p, profileErr := event.profileCli.GetByContact(ctx, &profilev1.GetByContactRequest{Contact: n.RecipientContactID})
		if profileErr != nil {
			logger.WithError(profileErr).Error("could not get profile by contact")
			if frame.ErrorIsNotFound(profileErr) {
				nStatus := models.NotificationStatus{
					NotificationID: n.GetID(),
					State:          int32(commonv1.STATE_INACTIVE),
					Status:         int32(commonv1.STATUS_FAILED),
					Extra: data.JSONMap{
						"error": "profile not found for contact",
						"step":  "lookup_profile_by_contact",
					},
				}
				nStatus.GenID(ctx)
				_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
				return nil
			}
			return profileErr
		}

		profileObj = p.GetData()
	} else {

		p, profileErr := event.profileCli.GetById(ctx, &profilev1.GetByIdRequest{Id: n.RecipientProfileID})
		if profileErr != nil {
			logger.WithError(profileErr).Error("could not get profile by id")
			if frame.ErrorIsNotFound(profileErr) {
				nStatus := models.NotificationStatus{
					NotificationID: n.GetID(),
					State:          int32(commonv1.STATE_INACTIVE),
					Status:         int32(commonv1.STATUS_FAILED),
					Extra: data.JSONMap{
						"error": "profile not found",
						"step":  "lookup_profile_by_id",
					},
				}
				nStatus.GenID(ctx)
				_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
				return nil
			}
			return profileErr
		}

		profileObj = p.GetData()
	}

	var channelTypes []string
	// Profile-addressed notifications without a contact are routed by the profile's reachable
	// contacts (verified first); the chosen contact is pinned on the notification so the
	// integration delivers to exactly that contact.
	var profileContacts []*profilev1.ContactObject
	resolveProfileContact := n.RecipientContactID == "" && n.RouteID == ""

	if resolveProfileContact {
		profileContacts = reachableContacts(profileObj)
		channelTypes = channelTypesForContacts(profileContacts)
	} else {
		contact := filterContactFromProfileByID(profileObj, n.RecipientContactID)

		var contactType profilev1.ContactType
		if contact != nil {
			contactType = contact.Type
		}
		channelTypes = channelTypesForContact(contactType)
	}

	route, channel, fallback, err := routeWithChannelPreference(ctx, event.routeRepo, models.RouteModeTransmit, n,
		channelTypes)
	if err != nil {
		logger.WithError(err).Error("could not route notification")

		if errors.Is(err, ErrNoRouteMatched) {
			extra := data.JSONMap{
				"error": err.Error(),
				"step":  "route_notification",
			}
			if resolveProfileContact && len(profileContacts) == 0 {
				extra["error"] = "recipient profile has no reachable contact"
				extra["step"] = "resolve_recipient_contact"
				extra["cause"] = err.Error()
			}
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra:          extra,
			}

			nStatus.GenID(ctx)

			err = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			if err != nil {
				logger.WithError(err).Error("could not emit status for save")
				return err
			}

			return nil

		}

		return err
	}

	n.RouteID = route.ID
	updateColumns := []string{"route_id", "notification_type"}
	if resolveProfileContact {
		if contact := contactForChannel(profileContacts, channel); contact != nil {
			n.RecipientContactID = contact.GetId()
			updateColumns = append(updateColumns, "recipient_contact_id")
		}
	}
	_, err = event.notificationRepo.Update(ctx, n, updateColumns...)
	if err != nil {
		logger.WithError(err).Error("could not save routed notification to db")
		return err
	}

	err = event.eventMan.Emit(ctx, NotificationOutQueueEvent, n.GetID())
	if err != nil {
		logger.WithError(err).Error("could not queue out notification")
		return err
	}

	statusExtra := data.JSONMap{
		"step":    "routed_for_queue",
		"channel": channel,
	}
	if fallback {
		statusExtra["channel_fallback"] = true
		logger.WithFields(map[string]any{"channel": channel, "route_id": route.ID}).
			Info("preferred channel has no route in partition, using fallback channel")
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_QUEUED),
		Extra:          statusExtra,
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
	if err != nil {
		logger.WithError(err).Error("could not emit status for save")
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}
