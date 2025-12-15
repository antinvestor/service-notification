package events

import (
	"context"
	"errors"
	"strings"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/events"
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

	logger := util.Log(ctx).WithField("type", event.Name())
	logger.WithField("payload", notificationId).Debug("handling event")

	n, err := event.notificationRepo.GetByID(ctx, notificationId)
	if err != nil {
		logger.WithError(err).Warn("could not get notification from db")
		return err
	}

	p, err := event.profileCli.GetById(ctx, connect.NewRequest(&profilev1.GetByIdRequest{Id: n.RecipientProfileID}))
	if err != nil {
		logger.WithError(err).WithField("profile_id", n.RecipientProfileID).Warn("could not get profile by id")
		return err
	}

	contact := filterContactFromProfileByID(p.Msg.GetData(), n.RecipientContactID)

	var contactType profilev1.ContactType

	if contact != nil {
		contactType = contact.Type
	}

	switch contactType {
	case profilev1.ContactType_MSISDN:
		n.NotificationType = models.RouteTypeSMSForm
	case profilev1.ContactType_EMAIL:
		n.NotificationType = models.RouteTypeEmailForm
	default:
		n.NotificationType = models.RouteTypeAny
	}

	route, err := routeNotification(ctx, event.routeRepo, models.RouteModeTransmit, n)
	if err != nil {
		logger.WithError(err).Error("could not route notification")

		if strings.Contains(err.Error(), "no routes matched for notification") {
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": err.Error(),
				},
			}

			nStatus.GenID(ctx)

			err = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
			if err != nil {
				logger.WithError(err).Warn("could not emit status for save")
				return err
			}

			return nil

		}

		return err
	}

	n.RouteID = route.ID
	_, err = event.notificationRepo.Update(ctx, n, "route_id")
	if err != nil {
		logger.WithError(err).Warn("could not save routed notification to db")
		return err
	}

	err = event.eventMan.Emit(ctx, NotificationOutQueueEvent, n.GetID())
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
	err = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit status for save")
		return err
	}

	return nil
}
