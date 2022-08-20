package events

import (
	"context"
	"errors"
	"github.com/antinvestor/service-notification/service/repository"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
)

func filterContactFromProfileByID(profile *profileV1.ProfileObject, contactID string) *profileV1.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetID() == contactID {
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
	return ""
}

func (event *NotificationInRoute) Validate(ctx context.Context, payload interface{}) error {
	_, ok := payload.(string)
	if !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationInRoute) Execute(ctx context.Context, payload interface{}) error {
	logger := logrus.WithField("payload", payload).WithField("type", event.Name())
	logger.Info("handling event")

	notificationID := payload.(string)

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationID)
	if err != nil {
		return err
	}

	route, err := routeRepository.GetByID(n.RouteID)
	if err != nil {
		return err
	}

	// Queue a message out for further processing by peripheral services
	err = event.Service.Publish(ctx, route.Uri, n)
	if err != nil {
		return err
	}

	return nil
}
