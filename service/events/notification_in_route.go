package events

import (
	"context"
	"errors"
	"fmt"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"strings"
)

func filterContactFromProfileByID(profile *profileV1.ProfileObject, contactID string) *profileV1.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetId() == contactID {
			return contact
		}
	}

	return nil
}

type NotificationInRoute struct {
	Service *frame.Service
}

func (event *NotificationInRoute) Name() string {
	return "notification.in.route"
}

func (event *NotificationInRoute) PayloadType() any {
	pType := ""
	return &pType
}

func (event *NotificationInRoute) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationInRoute) Execute(ctx context.Context, payload any) error {
	notificationID := *payload.(*string)
	logger := event.Service.L().WithField("payload", notificationID).WithField("type", event.Name())
	logger.Debug("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationID)
	if err != nil {
		return err
	}

	route, err := event.routeNotification(ctx, n)
	if err != nil {
		logger.WithError(err).Warn("could not route notification")

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

	evt := NotificationInQueue{}
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

func (event *NotificationInRoute) routeNotification(ctx context.Context, notification *models.Notification) (*models.Route, error) {

	if notification.RouteID != "" {

		return LoadRoute(ctx, event.Service, notification.RouteID)
	}

	routeRepository := repository.NewRouteRepository(ctx, event.Service)
	routes, err := routeRepository.GetByModeTypeAndPartitionID(
		models.RouteModeReceive, notification.NotificationType, notification.PartitionID)
	if err != nil {
		return nil, err
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes matched for notification : %s", notification.GetID())
	}

	route := routes[0]
	if len(routes) > 1 {
		route = event.selectRoute(ctx, routes)
	}

	return LoadRoute(ctx, event.Service, route.ID)

}

func LoadRoute(ctx context.Context, service *frame.Service, routeId string) (*models.Route, error) {

	if routeId == "" {
		return nil, fmt.Errorf("no route id provided")
	}

	routeRepository := repository.NewRouteRepository(ctx, service)

	route, err := routeRepository.GetByID(routeId)
	if err != nil {
		return nil, err
	}

	err = service.AddPublisher(ctx, route.ID, route.Uri)
	if err != nil {
		return route, err
	}

	return route, nil

}

func (event *NotificationInRoute) selectRoute(ctx context.Context, routes []*models.Route) *models.Route {
	//TODO: find a simple way of routing message mostly by settings
	// or contact and profile preferences

	return routes[0]
}
