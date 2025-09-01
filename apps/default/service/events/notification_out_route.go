package events

import (
	"context"
	"errors"
	"strings"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
)

// NotificationOutRouteEvent is the event name for routing outgoing notifications
const NotificationOutRouteEvent = "notification.out.route"

type NotificationOutRoute struct {
	Service          *frame.Service
	ProfileCli       *profilev1.ProfileClient
	NotificationRepo repository.NotificationRepository
}

// NewNotificationOutRoute creates a new NotificationOutRoute event handler
func NewNotificationOutRoute(ctx context.Context, service *frame.Service, profileCli *profilev1.ProfileClient) *NotificationOutRoute {
	return &NotificationOutRoute{
		Service:          service,
		ProfileCli:       profileCli,
		NotificationRepo: repository.NewNotificationRepository(ctx, service),
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

	logger := event.Service.Log(ctx).WithField("payload", notificationId).WithField("type", event.Name())
	logger.Debug("handling event")

	n, err := event.NotificationRepo.GetByID(ctx, notificationId)
	if err != nil {
		logger.WithError(err).Warn("could not get notification from db")
		return err
	}

	p, err := event.ProfileCli.GetProfileByID(ctx, n.RecipientProfileID)
	if err != nil {
		logger.WithError(err).WithField("profile_id", n.RecipientProfileID).Warn("could not get profile by id")
		return err
	}

	contact := filterContactFromProfileByID(p, n.RecipientContactID)

	var contactType profilev1.ContactType

	if contact != nil {
		contactType = contact.Type
	}

	switch contactType {
	case profilev1.ContactType_MSISDN:
		n.NotificationType = models.RouteTypeShortForm
	case profilev1.ContactType_EMAIL:
		n.NotificationType = models.RouteTypeLongForm
	default:
		n.NotificationType = models.RouteTypeAny
	}

	route, err := routeNotification(ctx, event.Service, models.RouteModeTransmit, n)
	if err != nil {
		logger.WithError(err).Error("could not route notification")

		if strings.Contains(err.Error(), "no routes matched for notification") {
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: frame.JSONMap{
					"error": err.Error(),
				},
			}

			nStatus.GenID(ctx)

			err = event.Service.Emit(ctx, NotificationStatusSaveEvent, nStatus)
			if err != nil {
				logger.WithError(err).Warn("could not emit status for save")
				return err
			}

			return nil

		}

		return err
	}

	n.RouteID = route.ID
	err = event.NotificationRepo.Save(ctx, n)
	if err != nil {
		logger.WithError(err).Warn("could not save routed notification to db")
		return err
	}

	err = event.Service.Emit(ctx, NotificationOutQueueEvent, n.GetID())
	if err != nil {
		logger.WithError(err).Warn("could not queue out notification")
		return err
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_QUEUED),
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = event.Service.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit status for save")
		return err
	}

	return nil
}
