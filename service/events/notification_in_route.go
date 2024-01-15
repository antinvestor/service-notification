package events

import (
	"context"
	"errors"
	"fmt"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
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

func (event *NotificationInRoute) PayloadType() interface{} {
	pType := ""
	return &pType
}

func (event *NotificationInRoute) Validate(ctx context.Context, payload interface{}) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationInRoute) Execute(ctx context.Context, payload interface{}) error {
	notificationID := *payload.(*string)
	logger := logrus.WithField("payload", notificationID).WithField("type", event.Name())
	logger.Info("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationID)
	if err != nil {
		return err
	}

	route, err := event.routeNotification(ctx, n)
	if err != nil {
		logger.WithError(err).Warn("could not route notification")
		return err
	}

	err = notificationRepo.Save(n)
	if err != nil {
		logger.WithError(err).Warn("could not save routed notification to db")
		return err
	}

	// Queue a message out for further processing by peripheral services
	err = event.Service.Publish(ctx, route.Uri, n)
	if err != nil {
		return err
	}

	return nil
}

func (event *NotificationInRoute) routeNotification(ctx context.Context, notification *models.Notification) (*models.Route, error) {
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	if notification.RouteID != "" {

		route, err := routeRepository.GetByID(notification.RouteID)
		if err != nil {
			return nil, err
		}

		return route, nil

	}

	routes, err := routeRepository.GetByModeTypeAndPartitionID(
		models.RouteModeReceive, notification.NotificationType, notification.PartitionID)
	if err != nil {
		return nil, err
	}

	if len(routes) > 0 {
		route := routes[0]
		if len(routes) > 1 {
			route = event.selectRoute(ctx, routes)
		}
		notification.RouteID = route.ID
		return route, nil
	} else {
		return nil, fmt.Errorf("no routes matched for notification : %s", notification.GetID())
	}
}

func (event *NotificationInRoute) selectRoute(ctx context.Context, routes []*models.Route) *models.Route {
	//TODO: find a simple way of routing message mostly by settings
	// or contact and profile preferences

	return routes[0]
}
