package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"

	"github.com/antinvestor/apis/common"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

func filterContactFromProfileByID(profile *papi.ProfileObject, contactID string) *papi.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetID() == contactID {
			return contact
		}
	}

	return nil
}

type NotificationSave struct {
	Service *frame.Service
}

func (e *NotificationSave) Name() string {
	return "notification.save"
}

func (e *NotificationSave) PayloadType() interface{} {
	return models.Notification{}
}

func (e *NotificationSave) Validate(ctx context.Context, payload interface{}) error {
	notification, ok := payload.(models.Notification)
	if !ok {
		return errors.New(" payload is not of type models.Notification")
	}

	if notification.GetID() == "" {
		return errors.New(" notification Id should already have been set ")
	}

	return nil
}

func (e *NotificationSave) Execute(ctx context.Context, payload interface{}) error {

	notification := payload.(models.Notification)
	notification.State = int32(common.STATE_ACTIVE)
	notification.Status = int32(common.STATUS_UNKNOWN)
	err := e.Service.DB(ctx, false).Save(notification).Error

	if err != nil {
		return err
	}

	if notification.OutBound {

		event := NotificationOutRoute{}
		err = e.Service.Emit(ctx, event.Name(), notification.GetID())
		if err != nil {
			return err
		}

	} else {

		event := NotificationInRoute{}
		err = e.Service.Emit(ctx, event.Name(), notification.GetID())
		if err != nil {
			return err
		}
	}

	return nil
}

type NotificationOutRoute struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

func (event *NotificationOutRoute) Name() string {
	return "notification.out.route"
}

func (event *NotificationOutRoute) PayloadType() interface{} {
	return ""
}

func (event *NotificationOutRoute) Validate(ctx context.Context, payload interface{}) error {
	_, ok := payload.(string)
	if !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutRoute) Execute(ctx context.Context, payload interface{}) error {

	notificationId := payload.(string)

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationId)
	if err != nil {
		return err
	}

	p, err := event.ProfileCli.GetProfileByID(ctx, n.ProfileID)
	if err != nil {
		return err
	}

	contact := filterContactFromProfileByID(p, n.ContactID)
	switch contact.Type {
	case papi.ContactType_PHONE:
		n.NotificationType = models.RouteTypeSms
	case papi.ContactType_EMAIL:
		n.NotificationType = models.RouteTypeEmail
	default:
		n.NotificationType = models.RouteTypeEmail
	}

	err = event.routeNotification(ctx, n)
	if err != nil {
		return err
	}

	n.State = int32(common.STATE_ACTIVE)
	n.Status = int32(common.STATUS_QUEUED)
	err = notificationRepo.Save(n)
	if err != nil {
		return err
	}

	evt := NotificationOutQueue{}
	err = event.Service.Emit(ctx, evt.Name(), n.GetID())
	if err != nil {
		return err
	}

	return nil
}

func (event *NotificationOutRoute) routeNotification(ctx context.Context, notification *models.Notification) error {

	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	routes, err := routeRepository.GetByModeTypeAndPartitionID(
		models.RouteModeTransmit, notification.NotificationType, notification.PartitionID)
	if err != nil {
		return err
	}

	if len(routes) > 0 {
		route := routes[0]
		if len(routes) > 1 {
			//TODO: find a simple way of routing message mostly by settings
			// or contact and profile preferences
		}
		notification.RouteID = route.ID
	} else {
		return errors.New(fmt.Sprintf("No routes matched for notification : %s", notification.GetID()))
	}

	return nil
}

type NotificationOutQueue struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

func (event *NotificationOutQueue) Name() string {
	return "notification.out.queue"
}

func (event *NotificationOutQueue) PayloadType() interface{} {
	return ""
}

func (event *NotificationOutQueue) Validate(ctx context.Context, payload interface{}) error {
	_, ok := payload.(string)
	if !ok {
		return errors.New(" payload is not of type string")
	}

	return nil
}

func (event *NotificationOutQueue) Execute(ctx context.Context, payload interface{}) error {

	notificationId := payload.(string)

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationId)
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

	n.State = int32(common.STATE_ACTIVE)
	n.Status = int32(common.STATUS_IN_PROCESS)

	err = notificationRepo.Save(n)
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
		templateMap[data.Type] = tmplBytes.String()

	}

	return templateMap, nil

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

	notificationId := payload.(string)

	notificationRepo := repository.NewNotificationRepository(ctx, event.Service)
	routeRepository := repository.NewRouteRepository(ctx, event.Service)

	n, err := notificationRepo.GetByID(notificationId)
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
