package events

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/data"
	"github.com/pitabwire/frame/v2/events"
	"github.com/pitabwire/frame/v2/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
)

// NotificationInQueueEvent is the event name for queuing incoming notifications
const NotificationInQueueEvent = "notification.in.queue"

type NotificationInQueue struct {
	qMan             queue.Manager
	eventMan         events.Manager
	profileCli       profilev1connect.ProfileServiceClient
	notificationRepo repository.NotificationRepository
	languageRepo     repository.LanguageRepository
	routeRepo        repository.RouteRepository
}

// NewNotificationInQueue creates a new NotificationInQueue event handler
func NewNotificationInQueue(_ context.Context, qMan queue.Manager, eventMan events.Manager,
	notificationRepo repository.NotificationRepository, languageRepo repository.LanguageRepository,
	routeRepo repository.RouteRepository, profileCli profilev1connect.ProfileServiceClient) *NotificationInQueue {

	return &NotificationInQueue{
		qMan:             qMan,
		eventMan:         eventMan,
		profileCli:       profileCli,
		notificationRepo: notificationRepo,
		languageRepo:     languageRepo,
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
	logger := util.Log(ctx).WithFields(map[string]any{"type": e.Name(), "notification_id": notificationID})
	defer logger.Release()
	logger.Debug("event handler started")

	n, err := e.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		logger.WithError(err).Error("could not get notification from db")
		return err
	}

	// Inbound consumers receive the same wire format integrations do for outbound:
	// the public proto encoding plus tenancy/route headers.
	var language *models.Language
	if n.LanguageID != "" {
		language, err = e.languageRepo.GetByID(ctx, n.LanguageID)
		if err != nil {
			logger.WithError(err).WithField("language_id", n.LanguageID).Warn("could not get language, publishing without it")
			language = nil
		}
	}

	binaryProto, err := proto.Marshal(n.ToAPI(nil, language, nil))
	if err != nil {
		logger.WithError(err).Error("could not marshal notification")
		nStatus := models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_INACTIVE),
			Status:         int32(commonv1.STATUS_FAILED),
			Extra: data.JSONMap{
				"error": err.Error(),
				"step":  "marshal_notification",
			},
		}
		nStatus.GenID(ctx)
		return e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
	}

	metadata := map[string]string{
		constants.TenantIDHeaderName:    n.TenantID,
		constants.PartitionIDHeaderName: n.PartitionID,
		constants.RouteIDHeaderName:     n.RouteID,
	}

	// Queue a message for further processing by peripheral services
	err = e.qMan.Publish(ctx, n.RouteID, binaryProto, metadata)
	if err != nil {

		logger.WithError(err).Error("could not publish to internal queue")

		if !frame.ErrorIsNotFound(err) || n.RouteID == "" {

			// Other publish error, not recoverable
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": err.Error(),
					"step":  "publish_to_queue",
				},
			}
			nStatus.GenID(ctx)
			_ = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			return nil
		}

		// Route publisher reference doesn't exist, try to load and register it
		route, loadErr := loadRoute(ctx, e.qMan, e.routeRepo, n.RouteID)
		if loadErr != nil {
			logger.WithError(loadErr).Error("could not load route")
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": loadErr.Error(),
					"step":  "load_route",
				},
			}
			nStatus.GenID(ctx)
			_ = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			return loadErr
		}
		logger.
			WithFields(map[string]any{"route_id": route.ID, "route_uri": route.Uri}).
			Debug("successfully loaded a route to use")

		// Retry publish after loading the route
		err = e.qMan.Publish(ctx, n.RouteID, binaryProto, metadata)
		if err != nil {
			logger.WithError(err).Error("could not publish to internal queue after route load")
			nStatus := models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": err.Error(),
					"step":  "publish_to_queue_retry",
				},
			}
			nStatus.GenID(ctx)
			_ = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
			return nil
		}
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_IN_PROCESS),
		Extra: data.JSONMap{
			"step": "queued_for_processing",
		},
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
	if err != nil {
		logger.WithError(err).Error("could not emit status save event")
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}
