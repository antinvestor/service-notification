package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/antinvestor/service-notification/service/repository/models"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"log"
	"text/template"
)

func routeOutboundNotification(channelRepo repository.ChannelRepository, notification *models.Notification) error {

	channels, err := channelRepo.GetByModeAndTypeAndProductID(models.ChannelModeTransmit, notification.Type, notification.ProductID)
	if err != nil {
		return err
	}

	if len(channels) > 0 {
		channel := channels[0]
		if len(channels) > 1 {
			//TODO: find a simple way of routing message mostly by settings
			// or contact and profile preferences
		}
		notification.ChannelID = channel.ID
	} else {
		return errors.New(fmt.Sprintf("No channels matched for routing message out : %s", notification.ProductID))
	}

	return nil
}

func formatOutboundNotification(templateRepository repository.TemplateRepository, notification *models.Notification) (map[string]string, error) {

	tmplDetail, err := templateRepository.GetByNameProductIDAndLanguageID(notification.TemplateID,
		notification.ProductID, notification.LanguageID)
	if err != nil {
		return nil, err
	}

	payload := make(map[string]string)
	data, err := notification.Payload.MarshalJSON()
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

func getContactFromProfileByContactID(profile *papi.ProfileObject, contactID string) *papi.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetID() == contactID {
			return contact
		}
	}

	return nil
}

func pushOutMessage(ctx context.Context, profile *papi.ProfileObject, contact *papi.ContactObject, notification *models.Notification, templateMap map[string]string) (map[string]string, error) {

	log.Printf("===========================================================")
	log.Printf(" We have successfully managed to get to post out ")
	log.Printf(" Contact details : %s", contact.Detail)
	log.Printf(" Notification details : %s", notification.ID)
	log.Printf(" Message details : %s", templateMap)
	log.Printf("===========================================================")
	responseMap := make(map[string]string)
	responseMap["message"] = templateMap["text"]
	responseMap["external_id"] = ""
	responseMap["transient_id"] = ""
	responseMap["state"] = ""
	responseMap["extra"] = ""
	return responseMap, nil
}

type MessageOutLoggedQueueHandler struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

// MessageOutLoggedQueueHandler Method subscribing to receive all messages that will be queued after being logged on their way out of the system.
func (m *MessageOutLoggedQueueHandler)Handle(ctx context.Context,  payload []byte, metadata map[string]string) error {

	notificationId, err := m.Service.QID(ctx, payload)
	if err != nil {
		return err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, m.Service)
	channelRepo := repository.NewChannelRepository(ctx, m.Service)

	n, err := notificationRepo.GetByID(notificationId)
	if err != nil {
		return err
	}

	p, err := m.ProfileCli.GetProfileByID(ctx, n.ProfileID)
	if err != nil {
		log.Printf("MessageOutLoggedQueueHandler -- Could not get the profile with id : %s : %v", n.ProfileID, err)
		return err
	}

	contact := getContactFromProfileByContactID(p, n.ContactID)
	switch contact.Type {
	case papi.ContactType_PHONE:
		n.Type = "sms"
	case papi.ContactType_EMAIL:
		n.Type = "email"
	default:
		n.Type = "unknown"
	}

	err = routeOutboundNotification(channelRepo, n)
	if err != nil {
		log.Printf("MessageOutLoggedQueueHandler -- Unable to route outbound notification by id %s : %v", n.GetID(), err)
		return err
	}

	err = notificationRepo.Save(n)
	if err != nil {
		log.Printf(" MessageOutLoggedQueueHandler -- Unable to update outbound notification by id %s : %v", n.GetID(), err)
		return err
	}

	payload, metadata, err = m.Service.QObject(ctx, n)
	queueName := fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, n.ChannelID)
	// Queue out a routed message for further processing
	err = m.Service.Publish(ctx, queueName, payload, metadata)
	if err != nil {
		log.Printf("MessageOutLoggedQueueHandler -- Could not publish channeled message out with id %s : %v ", n.GetID(), err)
		return err
	}

	return nil
}

type MessageOutRouteQueueHandler struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

func (m *MessageOutRouteQueueHandler) Handle(ctx context.Context, payload []byte, metadata map[string]string) error {

	notificationId, err := m.Service.QID(ctx, payload)
	if err != nil {
		return err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, m.Service)
	templateRepo := repository.NewTemplateRepository(ctx, m.Service)

	n, err := notificationRepo.GetByID(notificationId)
	if err != nil {
		log.Printf("MessageOutRouteQueueHandler -- Unable to obtain an outbound notification by queue id %s : %v", notificationId, err)
		return err
	}

	p, err := m.ProfileCli.GetProfileByID(ctx, n.ProfileID)
	if err != nil {
		log.Printf("MessageOutRouteQueueHandler -- Could not get the profile %s : %v", n.ProfileID, err)
		return err
	}

	contact := getContactFromProfileByContactID(p, n.ContactID)

	templateMap, err := formatOutboundNotification(templateRepo, n)
	if err != nil {
		log.Printf("MessageOutRouteQueueHandler -- Unable to format outbound notification by id %s : %v", n.GetID(), err)
		return err
	}

	response, err := pushOutMessage(ctx, p, contact, n, templateMap)
	if err != nil {
		log.Printf("MessageOutRouteQueueHandler -- Could not push out message with id %s : %v", n.GetID(), err)
		return err

	}

	n.Message = response["message"]
	n.ExternalID = response["external_id"]
	n.TransientID = response["transient_id"]
	n.State = response["state"]
	n.Extra = response["extra"]

	err = notificationRepo.Save(n)
	if err != nil {
		log.Printf("MessageOutRouteQueueHandler -- Unable to update message with id %s : %v", n.GetID(), err)
		return err
	}

	return nil
}

type MessageInLoggedQueueHandler struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

func (m *MessageInLoggedQueueHandler)Handle(ctx context.Context, payload []byte, metadata map[string]string) error {
	notificationId, err := m.Service.QID(ctx, payload)
	if err != nil {
		return err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, m.Service)

	_, err = notificationRepo.GetByID(notificationId)
	if err != nil {
		log.Printf("MessageInLoggedQueueHandler -- Unable to obtain an inbound notification by queue id %s : %v", notificationId, err)
		return err
	}

	return nil
}

type MessageInRoutedQueueHandler struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient
}

func (m *MessageInRoutedQueueHandler) Handle(ctx context.Context, payload []byte, metadata map[string]string) error {

	notificationId, err := m.Service.QID(ctx, payload)
	if err != nil {
		return err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, m.Service)

	_, err = notificationRepo.GetByID(notificationId)
	if err != nil {
		log.Printf("MessageInQueuedQueueHandler -- Unable to obtain an inbound notification by queue id %s : %v", notificationId, err)
		return err
	}

	return nil
}
