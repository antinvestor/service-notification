package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/antinvestor/apis/common"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
	"text/template"
)

type NotificationOutQueue struct {
	Service    *frame.Service
	ProfileCli *profileV1.ProfileClient
}

func (event *NotificationOutQueue) Name() string {
	return "notification.out.queue"
}

func (event *NotificationOutQueue) PayloadType() interface{} {
	pType := ""
	return &pType
}

func (event *NotificationOutQueue) Validate(ctx context.Context, payload interface{}) error {
	if _, ok := payload.(*string); !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutQueue) Execute(ctx context.Context, payload interface{}) error {
	notificationID := *payload.(*string)
	logger := logrus.WithField("payload", notificationID).WithField("type", event.Name())
	logger.Info("handling event")

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationID)
	if err != nil {
		return err
	}

	p, err := event.ProfileCli.GetProfileByID(ctx, n.ProfileID)
	if err != nil {
		return err
	}

	contact := filterContactFromProfileByID(p, n.ContactID)

	templateMap, err := event.formatOutboundNotification(ctx, n)
	if err != nil {
		return err
	}

	route, err := routeRepository.GetByID(n.RouteID)
	if err != nil {
		return err
	}

	message := map[string]interface{}{
		"profile": p,
		"contact": contact,
		"data":    templateMap,
	}

	// Queue a message for further processing by peripheral services
	err = event.Service.Publish(ctx, route.Uri, message)
	if err != nil {
		return err
	}

	log := event.Service.L()
	log.Info("===========================================================")
	log.Info(" We have successfully managed to get to post out ")
	log.Info(" Contact details : %s", contact.Detail)
	log.Info(" Notification details : %s", n.ID)
	log.Info(" Message details : %s", templateMap)
	log.Info("===========================================================")

	err = notificationRepo.Save(n)
	if err != nil {
		return err
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(common.STATE_ACTIVE),
		Status:         int32(common.STATUS_IN_PROCESS),
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

func (event *NotificationOutQueue) formatOutboundNotification(ctx context.Context, n *models.Notification) (map[string]string, error) {
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
