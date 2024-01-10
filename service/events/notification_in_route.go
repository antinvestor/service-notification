package events

import (
	"context"
	"errors"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
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
