package events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
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
	qMan     queue.Manager
	eventMan events.Manager

	notificationRepo repository.NotificationRepository
	routeRepo        repository.RouteRepository
}

// NewNotificationInRoute creates a new NotificationInRoute event handler
func NewNotificationInRoute(ctx context.Context, qMan queue.Manager, eventMan events.Manager, notificationRepo repository.NotificationRepository, routeRepo repository.RouteRepository) *NotificationInRoute {

	return &NotificationInRoute{
		qMan:             qMan,
		eventMan:         eventMan,
		notificationRepo: notificationRepo,
		routeRepo:        routeRepo,
	}
}

func (e *NotificationInRoute) Name() string {
	return NotificationInRouteEvent
}

func (e *NotificationInRoute) PayloadType() any {
	pType := ""
	return &pType
}

func (e *NotificationInRoute) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (e *NotificationInRoute) Execute(ctx context.Context, payload any) error {
	notificationID := *payload.(*string)
	logger := util.Log(ctx).WithField("type", e.Name()).WithField("notification_id", notificationID)
	defer logger.Release()
	logger.Debug("event handler started")

	n, err := e.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		logger.WithError(err).Error("could not get notification from db")
		return err
	}

	route, err := routeNotification(ctx, e.routeRepo, models.RouteModeReceive, n)
	if err != nil {
		logger.WithError(err).Error("could not route notification")

		if strings.Contains(err.Error(), "no routes matched for notification") {
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": err.Error(),
					"step":  "route_notification",
				},
			}

			nStatus.GenID(ctx)

			err = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			if err != nil {
				logger.WithError(err).Error("could not emit notification status save event")
				return err
			}

			return nil
		}

		return err
	}

	n.RouteID = route.ID

	_, err = e.notificationRepo.Update(ctx, n, "route_id")
	if err != nil {
		logger.WithError(err).Error("could not save routed notification to database")
		return err
	}

	err = e.eventMan.Emit(ctx, NotificationInQueueEvent, n.GetID())
	if err != nil {
		logger.WithError(err).Error("could not queue out notification")
		return err
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_QUEUED),
		Extra: data.JSONMap{
			"step": "routed_for_queue",
		},
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
	if err != nil {
		logger.WithError(err).Error("could not emit notification status save event")
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}

func routeNotification(ctx context.Context, routeRepository repository.RouteRepository, routeMode string, notification *models.Notification) (*models.Route, error) {

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

func loadRoute(ctx context.Context, qMan queue.Manager, routeRepository repository.RouteRepository, routeId string) (*models.Route, error) {

	if routeId == "" {
		return nil, fmt.Errorf("no route id provided")
	}

	route, err := routeRepository.GetByID(ctx, routeId)
	if err != nil {
		return nil, err
	}

	err = qMan.AddPublisher(ctx, route.ID, route.Uri)
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
