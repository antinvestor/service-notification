package business

import (
	"context"
	"encoding/json"
	n_api "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/antinvestor/service-notification/service/repository/models"
	p_api "github.com/antinvestor/service-profile-api"
	"github.com/go-errors/errors"
	"github.com/pitabwire/frame"
	"gorm.io/datatypes"
	"log"
	"strings"
	"time"
)

type notificationBusiness struct {
	service *frame.Service

	profileCli *p_api.ProfileClient

	notificationRepository repository.NotificationRepository
	templateRepository     repository.TemplateRepository
	languageRepository     repository.LanguageRepository

}

func (nb *notificationBusiness) QueueOut(ctx context.Context,
	productID string, message *n_api.MessageOut) (*n_api.StatusResponse, error) {

	payloadString, err := json.Marshal(message.GetMessageVariables())
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	var releaseDate time.Time
	if message.Autosend {
		releaseDate = time.Now()
	}

	languageCode := message.GetLanguage()
	if languageCode == "" {
		languageCode = "en"
	}
	language, err := nb.languageRepository.GetByCode(languageCode)
	if err != nil {
		return nil, err
	}

	template, err := nb.templateRepository.GetByNameProductIDAndLanguageID(
		message.GetMessageTemplete(), productID, language.ID)
	if err != nil {
		return nil, err
	}

	n := models.Notification{
		ContactID: message.GetContactID(),
		ProfileID: message.GetProfileID(),
		ProductID: productID,

		LanguageID: message.GetLanguage(),
		OutBound:   true,

		TemplateID: template.ID,
		Payload:    datatypes.JSON(payloadString),
		Type:       "",
		State:      "Logged",
		ReleasedAt: &releaseDate,
	}

	err = nb.notificationRepository.Save(&n)
	if err != nil {
		return nil, err
	}

	status := n_api.StatusResponse{NotificationID: n.ID, State: n.State}


	payload, meta, err := nb.service.QObject(ctx, &n)
	if err != nil {
		return &status, err
	}

	// Queue out message for further processing
	err = nb.service.Publish(ctx, config.QueueMessageOutLoggedName, payload, meta)
	if err != nil {
		log.Printf("Could not subscriptions message with id : %s - > %v", n.ID, err)
		return &status, err
	}

	return &status, nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *n_api.MessageIn) (*n_api.StatusResponse, error) {

	contactDetail := strings.Trim(message.GetContact(), " ")

	p, err := nb.profileCli.GetProfileByContact(ctx, contactDetail)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	var activeContact *p_api.ContactObject
	for _, contact := range p.GetContacts() {
		if contact.Detail == contactDetail {
			activeContact = contact
		}
	}

	if activeContact == nil {
		return nil, ErrorItemDoesNotExist
	}

	payloadString, err := json.Marshal(message.GetPayLoad())
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	releaseDate := time.Now()

	n := models.Notification{
		ContactID: activeContact.GetID(),
		ProfileID: p.GetID(),
		ProductID: message.GetProductID(),
		ChannelID: message.GetChannelID(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:    datatypes.JSON(payloadString),
		Type:       message.GetMessageType(),
		ReleasedAt: &releaseDate,
		ExternalID: message.GetNotificationID(),

		State: "Logged",
	}

	err = nb.notificationRepository.Save(&n)
	if err != nil {
		return nil, err
	}

	status := n_api.StatusResponse{NotificationID: n.ID, State: n.State, ExternalID: n.ExternalID}

	queueMap := make(map[string]string)
	queueMap["id"] = n.ID
	queueMap["product_id"] = n.ProductID
	queueID, err := json.Marshal(queueMap)
	if err != nil {
		return &status, errors.Wrap(err, 1)
	}
	// Queue in message for further processing
	err = nb.service.Publish(ctx, config.QueueMessageInLoggedName, queueID, nil)
	if err != nil {
		log.Printf("Could not subscriptions message with id : %s -> %v", n.ID, err)
		return &status, err
	}

	return &status, nil
}

func (nb *notificationBusiness) Status(ctx context.Context, productID string, statusReq *n_api.StatusRequest) (*n_api.StatusResponse, error) {

	n, err := nb.notificationRepository.GetByIDAndProductID(statusReq.NotificationID, productID)
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}

	status := n_api.StatusResponse{
		NotificationID: n.ID,
		State:          n.State,
		TransientID:    n.TransientID,
		ExternalID:     n.ExternalID,
		ExternalStatus: n.Extra,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Release(ctx context.Context, productID string, releaseReq *n_api.ReleaseRequest) (*n_api.StatusResponse, error) {

	n, err := nb.notificationRepository.GetByIDAndProductID(releaseReq.NotificationID, productID)
	if err != nil {
		return nil, err
	}

	status := n_api.StatusResponse{
		NotificationID: n.ID,
		State:          n.State,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Search(ctx context.Context, productID string, search *n_api.SearchRequest, stream n_api.NotificationService_SearchServer) error {

	notificationList, err := nb.notificationRepository.Search(search.GetNotificationID(), productID)
	if err != nil {
		return err
	}

	for _, n := range notificationList {

		payload := make(map[string]string)
		payloadValue, _ := n.Payload.MarshalJSON()
		err = json.Unmarshal(payloadValue, &payload)
		if err != nil {
			log.Printf(" Search -- there is a problem : %+v ", err)
		}
		result := n_api.SearchResponse{
			NotificationID: n.ID,
			ProfileID:      n.ProfileID,
			ContactID:      n.ContactID,
			ProductID:      n.ProductID,
			Language:       n.LanguageID,
			MessageType:    n.Type,
			PayLoad:        payload,
			Outbound:       n.OutBound,
			State:          n.State,
			Released:       n.IsReleased(),
			TransientID:    n.TransientID,
			ExternalID:     n.ExternalID,
			ExternalStatus: n.Extra,
		}

		err = stream.Send(&result)
		if err != nil {
			log.Printf(" Search -- unable to send a result see %v", errors.Wrap(err, 1))
		}
	}

	return nil
}
