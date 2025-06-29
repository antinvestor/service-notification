package events

import (
	"context"
	"errors"
	"strings"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
)

type NotificationInQueue struct {
	Service    *frame.Service
	ProfileCli *profileV1.ProfileClient
}

func (event *NotificationInQueue) Name() string {
	return "notification.in.queue"
}

func (event *NotificationInQueue) PayloadType() any {
	pType := ""
	return &pType
}

func (event *NotificationInQueue) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationInQueue) Execute(ctx context.Context, payload any) error {
	notificationID := *payload.(*string)
	logger := event.Service.Log(ctx).WithField("payload", notificationID).WithField("type", event.Name())
	logger.Debug("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Queue a message for further processing by peripheral services
	err = event.Service.Publish(ctx, n.RouteID, n)
	if err != nil {

		if !strings.Contains(err.Error(), "reference does not exist") {

			if n.RouteID != "" {
				route, err0 := loadRoute(ctx, event.Service, n.RouteID)
				if err0 != nil {
					return err0
				}
				logger.WithField("route", route).Debug("loading route")
			}

			return err
		}
	}

	logger.
		WithField("notification", n.ID).
		WithField("route", n.RouteID).
		Debug(" Successfully routed in message ")

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_IN_PROCESS),
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	eventStatus := NotificationStatusSave{}
	err = event.Service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		return err
	}

	return nil
}
