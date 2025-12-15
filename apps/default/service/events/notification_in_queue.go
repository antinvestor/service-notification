package events

import (
	"context"
	"errors"
	"strings"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
)

// NotificationInQueueEvent is the event name for queuing incoming notifications
const NotificationInQueueEvent = "notification.in.queue"

type NotificationInQueue struct {
	qMan             queue.Manager
	eventMan         events.Manager
	profileCli       profilev1connect.ProfileServiceClient
	notificationRepo repository.NotificationRepository
	routeRepo        repository.RouteRepository
}

// NewNotificationInQueue creates a new NotificationInQueue event handler
func NewNotificationInQueue(_ context.Context, qMan queue.Manager, eventMan events.Manager, notificationRepo repository.NotificationRepository, routeRepo repository.RouteRepository, profileCli profilev1connect.ProfileServiceClient) *NotificationInQueue {

	return &NotificationInQueue{
		qMan:             qMan,
		eventMan:         eventMan,
		profileCli:       profileCli,
		notificationRepo: notificationRepo,
		routeRepo:        routeRepo,
	}
}

func (e *NotificationInQueue) Name() string {
	return NotificationInQueueEvent
}

func (e *NotificationInQueue) PayloadType() any {
	pType := ""
	return &pType
}

func (e *NotificationInQueue) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (e *NotificationInQueue) Execute(ctx context.Context, payload any) error {
	notificationID := *payload.(*string)
	logger := util.Log(ctx).WithField("type", e.Name())
	logger.WithField("payload", notificationID).Debug("handling e")

	n, err := e.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Queue a message for further processing by peripheral services
	err = e.qMan.Publish(ctx, n.RouteID, n)
	if err != nil {

		if !strings.Contains(err.Error(), "reference does not exist") {

			if n.RouteID != "" {
				route, err0 := loadRoute(ctx, e.qMan, e.routeRepo, n.RouteID)
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
	err = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	if err != nil {
		return err
	}

	return nil
}
