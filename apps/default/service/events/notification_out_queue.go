package events

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"text/template"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
)

// NotificationOutQueueEvent is the event name for queuing outgoing notifications
const NotificationOutQueueEvent = "notification.out.queue"

type NotificationOutQueue struct {
	Service                *frame.Service
	ProfileCli             *profilev1.ProfileClient
	NotificationRepo       repository.NotificationRepository
	NotificationStatusRepo repository.NotificationStatusRepository
	LanguageRepo           repository.LanguageRepository
	TemplateDataRepo       repository.TemplateDataRepository
}

// NewNotificationOutQueue creates a new NotificationOutQueue event handler
func NewNotificationOutQueue(ctx context.Context, service *frame.Service, profileCli *profilev1.ProfileClient) *NotificationOutQueue {
	return &NotificationOutQueue{
		Service:                service,
		ProfileCli:             profileCli,
		NotificationRepo:       repository.NewNotificationRepository(ctx, service),
		NotificationStatusRepo: repository.NewNotificationStatusRepository(ctx, service),
		LanguageRepo:           repository.NewLanguageRepository(ctx, service),
		TemplateDataRepo:       repository.NewTemplateDataRepository(ctx, service),
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

	logger := event.Service.Log(ctx).WithField("payload", notificationID).WithField("type", event.Name())
	logger.Debug("handling event")

	n, err := event.NotificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	nStatus, err := event.NotificationStatusRepo.GetByID(ctx, n.StatusID)
	if err != nil {
		logger.WithError(err).WithField("status_id", n.StatusID).Warn(" could not get status")
		return err
	}

	language, err := event.LanguageRepo.GetByID(ctx, n.LanguageID)
	if err != nil {
		logger.WithError(err).WithField("language_id", n.LanguageID).Warn(" could not get language")
		return err
	}

	var templateMap map[string]string
	templateMap, err = event.formatOutboundNotification(ctx, logger, n)
	if err != nil {
		return err
	}

	apiNotification := n.ToApi(nStatus, language, templateMap)

	binaryProto, err := proto.Marshal(apiNotification)
	if err != nil {
		return err
	}

	// Queue a message for further processing by peripheral services
	err = event.Service.Publish(ctx, n.RouteID, binaryProto)
	if err != nil {

		if !strings.Contains(err.Error(), "reference does not exist") {

			if n.RouteID != "" {
				_, err = loadRoute(ctx, event.Service, n.RouteID)
				if err != nil {
					return err
				}
			}

			return err
		}

		return err
	}

	logger.WithField("notification_id", n.GetID()).
		WithField("route", n.RouteID).
		WithField("message", templateMap).
		Debug(" We have successfully queued out message")

	err = event.NotificationRepo.Save(ctx, n)
	if err != nil {
		return err
	}

	nStatus = &models.NotificationStatus{
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

func (event *NotificationOutQueue) formatOutboundNotification(ctx context.Context, logger *util.LogEntry, n *models.Notification) (map[string]string, error) {

	templateMap := make(map[string]string)

	if n.Message != "" {
		templateMap = map[string]string{"default": n.Message}
		return templateMap, nil
	}

	if n.TemplateID == "" {
		return nil, errors.New("no template id specified")
	}

	tmplDataList, err0 := event.TemplateDataRepo.GetByTemplateIDAndLanguage(ctx, n.TemplateID, n.LanguageID)
	if err0 != nil {
		logger.WithError(err0).
			WithField("template id", n.TemplateID).
			WithField("language id", n.LanguageID).Error(" could not get template data")
		tmplDataList = []*models.TemplateData{}
	}

	payload := frame.DBPropertiesToMap(n.Payload)

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
