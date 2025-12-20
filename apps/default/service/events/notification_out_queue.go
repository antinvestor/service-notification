package events

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"text/template"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/antinvestor/service-notification/internal/constants"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
)

// NotificationOutQueueEvent is the event name for queuing outgoing notifications
const NotificationOutQueueEvent = "notification.out.queue"

type NotificationOutQueue struct {
	qMan     queue.Manager
	eventMan events.Manager

	ProfileCli             profilev1connect.ProfileServiceClient
	NotificationRepo       repository.NotificationRepository
	NotificationStatusRepo repository.NotificationStatusRepository
	LanguageRepo           repository.LanguageRepository
	TemplateDataRepo       repository.TemplateDataRepository
	routeRepo              repository.RouteRepository
}

// NewNotificationOutQueue creates a new NotificationOutQueue event handler
func NewNotificationOutQueue(ctx context.Context, qMan queue.Manager, eventMan events.Manager, profileCli profilev1connect.ProfileServiceClient, notificationRepo repository.NotificationRepository, notificationStatusRepo repository.NotificationStatusRepository, languageRepo repository.LanguageRepository, templateDataRepo repository.TemplateDataRepository, routeRepo repository.RouteRepository) *NotificationOutQueue {

	return &NotificationOutQueue{
		qMan:                   qMan,
		eventMan:               eventMan,
		ProfileCli:             profileCli,
		NotificationRepo:       notificationRepo,
		NotificationStatusRepo: notificationStatusRepo,
		LanguageRepo:           languageRepo,
		TemplateDataRepo:       templateDataRepo,
		routeRepo:              routeRepo,
	}
}

func (event *NotificationOutQueue) Name() string {
	return NotificationOutQueueEvent
}

func (event *NotificationOutQueue) PayloadType() any {
	pType := ""
	return &pType
}

func (event *NotificationOutQueue) Validate(ctx context.Context, payload any) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutQueue) Execute(ctx context.Context, payload any) error {
	notificationID := *payload.(*string)

	logger := util.Log(ctx).WithField("type", event.Name()).WithField("notification_id", notificationID)
	defer logger.Release()
	logger.Debug("event handler started")

	n, err := event.NotificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		logger.WithError(err).Warn("could not get notification from db")
		return err
	}

	nStatus, err := event.NotificationStatusRepo.GetByID(ctx, n.StatusID)
	if err != nil {
		logger.WithError(err).WithField("status_id", n.StatusID).Warn("could not get status")
		return err
	}

	language, err := event.LanguageRepo.GetByID(ctx, n.LanguageID)
	if err != nil {
		logger.WithError(err).WithField("language_id", n.LanguageID).Warn("could not get language")
		return err
	}

	var templateMap map[string]string
	templateMap, err = event.formatOutboundNotification(ctx, logger, n)
	if err != nil {
		logger.WithError(err).WithField("notification_id", n.GetID()).Error("could not format outbound notification")

		nStatus = &models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_INACTIVE),
			Status:         int32(commonv1.STATUS_FAILED),
			Extra: data.JSONMap{
				"error": err.Error(),
				"step":  "format_outbound_notification",
			},
		}

		nStatus.GenID(ctx)
		return event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	}

	apiNotification := n.ToAPI(nStatus, language, templateMap)

	binaryProto, err := proto.Marshal(apiNotification)
	if err != nil {
		logger.WithError(err).WithField("notification_id", n.GetID()).Error("could not marshal notification")

		nStatus = &models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_INACTIVE),
			Status:         int32(commonv1.STATUS_FAILED),
			Extra: data.JSONMap{
				"error": err.Error(),
				"step":  "marshal_notification",
			},
		}

		nStatus.GenID(ctx)

		return event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	}

	metadata := map[string]string{
		constants.TenantIDHeaderName:    n.TenantID,
		constants.PartitionIDHeaderName: n.PartitionID,
		constants.RouteIDHeaderName:     n.RouteID,
	}

	if n.RouteID == "" {
		logger.Error("message is not routed correctly")

		nStatus = &models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_INACTIVE),
			Status:         int32(commonv1.STATUS_FAILED),
			Extra: data.JSONMap{
				"error": "message was not routed correctly",
				"step":  "validate_route",
			},
		}

		nStatus.GenID(ctx)

		return event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	}

	// Queue a message for further processing by peripheral services
	err = event.qMan.Publish(ctx, n.RouteID, binaryProto, metadata)
	if err != nil {

		logger.WithError(err).Error("could not publish to external queue")

		if strings.Contains(err.Error(), "reference does not exist") && n.RouteID != "" {
			// Route publisher reference doesn't exist, try to load and register it
			route, loadErr := loadRoute(ctx, event.qMan, event.routeRepo, n.RouteID)
			if loadErr != nil {
				logger.WithError(loadErr).Error("could not load route")
				nStatus = &models.NotificationStatus{
					NotificationID: n.GetID(),
					State:          int32(commonv1.STATE_INACTIVE),
					Status:         int32(commonv1.STATUS_FAILED),
					Extra: data.JSONMap{
						"error": loadErr.Error(),
						"step":  "load_route",
					},
				}
				nStatus.GenID(ctx)
				_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
				return loadErr
			}
			logger.WithField("route_uri", route.Uri).Debug("successfully loaded a route to use")

			// Retry publish after loading the route
			err = event.qMan.Publish(ctx, n.RouteID, binaryProto, metadata)
			if err != nil {
				logger.WithError(err).Error("could not publish to external queue after route load")
				nStatus = &models.NotificationStatus{
					NotificationID: n.GetID(),
					State:          int32(commonv1.STATE_INACTIVE),
					Status:         int32(commonv1.STATUS_FAILED),
					Extra: data.JSONMap{
						"error": err.Error(),
						"step":  "publish_to_queue_retry",
					},
				}
				nStatus.GenID(ctx)
				_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
				return nil
			}
		} else {
			// Other publish error, not recoverable
			nStatus = &models.NotificationStatus{
				NotificationID: n.GetID(),
				State:          int32(commonv1.STATE_INACTIVE),
				Status:         int32(commonv1.STATUS_FAILED),
				Extra: data.JSONMap{
					"error": err.Error(),
					"step":  "publish_to_queue",
				},
			}
			nStatus.GenID(ctx)
			_ = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
			return nil
		}
	}

	nStatus = &models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_ACTIVE),
		Status:         int32(commonv1.STATUS_IN_PROCESS),
		Extra: data.JSONMap{
			"step": "queued_for_delivery",
		},
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = event.eventMan.Emit(ctx, NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Error("could not emit status save event")
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}

func (event *NotificationOutQueue) formatOutboundNotification(ctx context.Context, logger *util.LogEntry, n *models.Notification) (map[string]string, error) {

	templateMap := make(map[string]string)

	if n.Message != "" {
		templateMap = map[string]string{"default": n.Message}
		return templateMap, nil
	}

	if n.TemplateID == "" {
		return nil, errors.New("no template id specified")
	}

	tmplDataList, err0 := event.TemplateDataRepo.GetByTemplateIDAndLanguage(ctx, n.LanguageID, n.TemplateID)
	if err0 != nil {
		logger.WithError(err0).
			WithField("template id", n.TemplateID).
			WithField("language id", n.LanguageID).Error("could not get template data")
		tmplDataList = []*models.TemplateData{}
	}

	payload := n.Payload

	for _, templateData := range tmplDataList {

		tmpl, err := template.New("message_out").Parse(templateData.Detail)
		if err != nil {
			return nil, err
		}

		var tmplBytes bytes.Buffer
		err = tmpl.Execute(&tmplBytes, payload)
		if err != nil {
			return nil, err
		}
		templateMap[templateData.Type] = tmplBytes.String()
	}

	return templateMap, nil

}
