package events

import (
	"context"
	"errors"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"strings"
)

type NotificationOutRoute struct {
	Service    *frame.Service
	ProfileCli *profileV1.ProfileClient
}

func (event *NotificationOutRoute) Name() string {
	return "notification.out.route"
}

func (event *NotificationOutRoute) PayloadType() interface{} {
	pType := ""
	return &pType
}

func (event *NotificationOutRoute) Validate(ctx context.Context, payload interface{}) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutRoute) Execute(ctx context.Context, payload interface{}) error {

	notificationId := *payload.(*string)

	logger := event.Service.L().WithField("payload", notificationId).WithField("type", event.Name())
	logger.Debug("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationId)
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
	switch contact.Type {
	case profileV1.ContactType_PHONE:
		n.NotificationType = models.RouteTypeShortForm
	case profileV1.ContactType_EMAIL:
		n.NotificationType = models.RouteTypeLongForm
	default:
		n.NotificationType = models.RouteTypeAny
	}

	route, err := event.routeNotification(ctx, n)
	if err != nil {
		logger.WithError(err).Error("could not route notification")

		if strings.Contains(err.Error(), "no routes matched for notification") {
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: frame.DBPropertiesFromMap(map[string]string{
					"error": err.Error(),
				}),
			}

			nStatus.GenID(ctx)

			eventStatus := NotificationStatusSave{}
			err = event.Service.Emit(ctx, eventStatus.Name(), nStatus)
			if err != nil {
				logger.WithError(err).Warn("could not emit status for save")
				return err
			}

			return nil

		}

		return err
	}

	n.RouteID = route.ID
	err = notificationRepo.Save(n)
	if err != nil {
		logger.WithError(err).Warn("could not save routed notification to db")
		return err
	}

	evt := NotificationOutQueue{}
	err = event.Service.Emit(ctx, evt.Name(), n.GetID())
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
	eventStatus := NotificationStatusSave{}
	err = event.Service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit status for save")
		return err
	}

	return nil
}

func (event *NotificationOutRoute) routeNotification(ctx context.Context, notification *models.Notification) (*models.Route, error) {
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	if notification.RouteID != "" {

		route, err := routeRepository.GetByID(notification.RouteID)
		if err != nil {
			return nil, err
		}

		err = event.Service.AddPublisher(ctx, route.ID, route.Uri)
		if err != nil {
			return nil, err
		}

		return route, nil

	}

	routes, err := routeRepository.GetByModeTypeAndPartitionID(
		models.RouteModeTransmit, notification.NotificationType, notification.PartitionID)
	if err != nil {
		return nil, err
	}

	var route *models.Route
	route, err = event.selectRoute(ctx, routes)
	if err != nil {
		return nil, err
	}

	err = event.Service.AddPublisher(ctx, route.ID, route.Uri)
	if err != nil {
		return nil, err
	}

	return route, nil

}

func (event *NotificationOutRoute) selectRoute(_ context.Context, routes []*models.Route) (*models.Route, error) {
	//TODO: find a simple way of routing message mostly by settings
	// or contact and profile preferences

	if len(routes) == 0 {
		return nil, errors.New("no routes matched for notification")
	}

	return routes[0], nil
}
