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

// NotificationInQueueEvent is the event name for queuing incoming notifications
const NotificationInQueueEvent = "notification.in.queue"

type NotificationInQueue struct {
	Service          *frame.Service
	ProfileCli       *profilev1.ProfileClient
	NotificationRepo repository.NotificationRepository
}

// NewNotificationInQueue creates a new NotificationInQueue event handler
func NewNotificationInQueue(ctx context.Context, service *frame.Service, profileCli *profilev1.ProfileClient) *NotificationInQueue {
	return &NotificationInQueue{
		Service:          service,
		ProfileCli:       profileCli,
		NotificationRepo: repository.NewNotificationRepository(ctx, service),
	}
}

func (event *NotificationInQueue) Name() string {
	return NotificationInQueueEvent
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

	n, err := event.NotificationRepo.GetByID(ctx, notificationID)
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
	err = event.Service.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	if err != nil {
		return err
	}

	return nil
}
