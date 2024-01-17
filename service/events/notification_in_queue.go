package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
	"text/template"
)

type NotificationInQueue struct {
	Service    *frame.Service
	ProfileCli *profileV1.ProfileClient
}

func (event *NotificationInQueue) Name() string {
	return "notification.in.queue"
}

func (event *NotificationInQueue) PayloadType() interface{} {
	pType := ""
	return &pType
}

func (event *NotificationInQueue) Validate(ctx context.Context, payload interface{}) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationInQueue) Execute(ctx context.Context, payload interface{}) error {
	notificationID := *payload.(*string)
	logger := logrus.WithField("payload", notificationID).WithField("type", event.Name())
	logger.Info("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationID)
	if err != nil {
		return err
	}

	// Queue a message for further processing by peripheral services
	err = event.Service.Publish(ctx, n.RouteID, n)
	if err != nil {
		return err
	}

	log := event.Service.L()
	log.
		WithField("notification", n.ID).
		Info(" Successfully routed in message ")

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

func (event *NotificationInQueue) formatOutboundNotification(ctx context.Context, n *models.Notification) (map[string]string, error) {

	if n.TemplateID == "" {
		return nil, errors.New("No template id specified")
	}

	templateRepository := repository.NewTemplateRepository(ctx, event.Service)

	tmplDetail, err := templateRepository.GetByID(n.TemplateID)
	if err != nil {
		return nil, err
	}

	payload := make(map[string]string)
	data, err := n.Payload.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &payload)
	if err != nil {
		return nil, err
	}

	templateMap := make(map[string]string)

	for _, data := range tmplDetail.DataList {

		tmpl, err := template.New("message_out").Parse(data.Detail)
		if err != nil {
			return nil, err
		}

		var tmplBytes bytes.Buffer
		err = tmpl.Execute(&tmplBytes, payload)
		if err != nil {
			return nil, err
		}
		templateMap[data.Type] = tmplBytes.String()

	}

	return templateMap, nil

}
