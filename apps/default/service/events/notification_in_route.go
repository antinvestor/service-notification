package events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
)

// NotificationInRouteEvent is the event name for routing incoming notifications
const NotificationInRouteEvent = "notification.in.route"

func filterContactFromProfileByID(profile *profilev1.ProfileObject, contactID string) *profilev1.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetId() == contactID {
			return contact
		}
	}

	return nil
}

type NotificationInRoute struct {
	Service          *frame.Service
	NotificationRepo repository.NotificationRepository
}

// NewNotificationInRoute creates a new NotificationInRoute event handler
func NewNotificationInRoute(ctx context.Context, service *frame.Service) *NotificationInRoute {
	return &NotificationInRoute{
		Service:          service,
		NotificationRepo: repository.NewNotificationRepository(ctx, service),
	}
}

func (event *NotificationInRoute) Name() string {
	return NotificationInRouteEvent
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
	logger := event.Service.Log(ctx).WithField("payload", notificationID).WithField("type", event.Name())
	logger.Debug("handling event")

	n, err := event.NotificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	route, err := routeNotification(ctx, event.Service, models.RouteModeReceive, n)
	if err != nil {
		logger.WithError(err).Warn("could not route notification")

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

	err = event.Service.Emit(ctx, NotificationInQueueEvent, n.GetID())
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

func routeNotification(ctx context.Context, service *frame.Service, routeMode string, notification *models.Notification) (*models.Route, error) {

	routeRepository := repository.NewRouteRepository(ctx, service)
	if notification.RouteID != "" {
		route, err := routeRepository.GetByID(ctx, notification.RouteID)
		if err != nil {
			return nil, err
		}
		return route, nil
	}

	routes, err := routeRepository.GetByModeTypeAndPartitionID(ctx,
		routeMode, notification.NotificationType, notification.PartitionID)
	if err != nil {
		return nil, err
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes matched for notification : %s", notification.GetID())
	}

	route := routes[0]
	if len(routes) > 1 {
		route, err = selectRoute(ctx, routes)
		if err != nil {
			return nil, err
		}
	}

	return route, nil

}

func loadRoute(ctx context.Context, service *frame.Service, routeId string) (*models.Route, error) {

	if routeId == "" {
		return nil, fmt.Errorf("no route id provided")
	}

	routeRepository := repository.NewRouteRepository(ctx, service)

	route, err := routeRepository.GetByID(ctx, routeId)
	if err != nil {
		return nil, err
	}

	err = service.AddPublisher(ctx, route.ID, route.Uri)
	if err != nil {
		return route, err
	}

	return route, nil

}

func selectRoute(_ context.Context, routes []*models.Route) (*models.Route, error) {
	// TODO: find a simple way of routing message mostly by settings
	// or contact and profile preferences
	if len(routes) == 0 {
		return nil, errors.New("no routes matched for notification")
	}
	return routes[0], nil
}
