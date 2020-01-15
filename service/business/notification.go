package business

import (
	"antinvestor.com/service/notification/grpc/notification"
	profile "antinvestor.com/service/notification/grpc/profile"
	"antinvestor.com/service/notification/service/repository"
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"context"
	"encoding/json"
	"github.com/jinzhu/gorm/dialects/postgres"
	"strings"
	"time"
)

type notificationBusiness struct {
	env                    *utils.Env
	notificationRepository repository.NotificationRepository
	templateRepository repository.TemplateRepository
	languageRepository repository.LanguageRepository
}

func (nb *notificationBusiness) QueueOut(
	ctx context.Context,
	productID string,
	message *notification.MessageOut) (*notification.StatusResponse, error) {

	payloadString, err := json.Marshal(message.GetMessageVariables())
	if err != nil {
		return nil, err
	}

	var releaseDate time.Time
	if message.Autosend {
		releaseDate = time.Now()
	}

	languageCode := message.GetLanguage()
	if languageCode == ""{
		languageCode = "en"
	}
	language, err := nb.languageRepository.GetByCode(languageCode)
	if err != nil{
		return nil, err
	}

	template, err := nb.templateRepository.GetByNameProductIDAndLanguageID(
		message.GetMessageTemplete(), productID, language.LanguageID)
	if err != nil{
		return nil, err
	}

	n := models.Notification{
		ContactID: message.GetContactID(),
		ProfileID: message.GetProfileID(),
		ProductID: productID,

		LanguageID: message.GetLanguage(),
		OutBound:   true,

		TemplateID:   template.TempleteID,
		Payload:    postgres.Jsonb{payloadString},
		Type:       "",
		State:      "Logged",
		ReleasedAt: &releaseDate,
	}

	err = nb.notificationRepository.Save(&n)
	if err != nil {
		return nil, err
	}

	status := notification.StatusResponse{NotificationID: n.NotificationID, State: n.State}

	queueID, err := utils.QueueMakeID(ctx, n.NotificationID)
	if err != nil {
		return &status, err
	}
	// Queue out message for further processing
	err = nb.env.Queue.Publish(utils.ConfigQueueMessageOutLoggedName, queueID)
	if err != nil {
		nb.env.Logger.WithError(err).Errorf("Could not subscriptions message with id : %s", n.NotificationID)
		return &status, err
	}

	return &status, nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notification.MessageIn) (*notification.StatusResponse, error) {

	contactDetail := strings.Trim(message.GetContact(), " ")

	p, err := profile.GetOrCreateProfileByContactDetail(ctx, nb.env.GetProfileServiceConn(), contactDetail)
	if err != nil {
		return nil, err
	}

	var activeContact *profile.ContactObject
	for _, contact := range p.GetContacts() {
		if contact.Detail == contactDetail {
			activeContact = contact
		}
	}

	if activeContact == nil {
		return nil, notification.ErrorItemDoesNotExist
	}

	payloadString, err := json.Marshal(message.GetPayLoad())
	if err != nil {
		return nil, err
	}

	releaseDate := time.Now()

	n := models.Notification{
		ContactID: activeContact.GetID(),
		ProfileID: p.GetID(),
		ProductID: message.GetProductID(),
		ChannelID: message.GetChannelID(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:    postgres.Jsonb{payloadString},
		Type:       message.GetMessageType(),
		ReleasedAt: &releaseDate,
		ExternalID: message.GetNotificationID(),

		State: "Logged",
	}

	err = nb.notificationRepository.Save(&n)
	if err != nil {
		return nil, err
	}

	status := notification.StatusResponse{NotificationID: n.NotificationID, State: n.State, ExternalID: n.ExternalID}

	queueMap := make(map[string]string)
	queueMap["id"] = n.NotificationID
	queueMap["product_id"] = n.ProductID
	queueID, err := json.Marshal(queueMap)
	if err != nil {
		return &status, err
	}
	// Queue in message for further processing
	err = nb.env.Queue.Publish(utils.ConfigQueueMessageInLoggedName, queueID)
	if err != nil {
		nb.env.Logger.WithError(err).Errorf("Could not subscriptions message with id : %s", n.NotificationID)
		return &status, err
	}

	return &status, nil
}

func (nb *notificationBusiness) Status(ctx context.Context, productID string, statusReq *notification.StatusRequest) (*notification.StatusResponse, error) {

	n, err := nb.notificationRepository.GetByIDAndProductID(statusReq.NotificationID, productID)
	if err != nil {
		return nil, err
	}

	status := notification.StatusResponse{
		NotificationID: n.NotificationID,
		State:          n.State,
		TransientID:    n.TransientID,
		ExternalID:     n.ExternalID,
		ExternalStatus: n.Extra,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Release(ctx context.Context, productID string, releaseReq *notification.ReleaseRequest) (*notification.StatusResponse, error) {

	n, err := nb.notificationRepository.GetByIDAndProductID(releaseReq.NotificationID, productID)
	if err != nil {
		return nil, err
	}

	status := notification.StatusResponse{
		NotificationID: n.NotificationID,
		State:          n.State,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Search(ctx context.Context, productID string, search *notification.SearchRequest, stream notification.NotificationService_SearchServer) error {

	notificationList, err := nb.notificationRepository.Search(search.GetNotificationID(), productID)
	if err != nil {
		return err
	}

	for _, n := range notificationList {

		payload := make(map[string]string)
		json.Unmarshal(n.Payload.RawMessage, payload)

		result := notification.SearchResponse{
			NotificationID: n.NotificationID,
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

		stream.Send(&result)
	}

	return nil
}
