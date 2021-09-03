package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/antinvestor/apis/common"
	napi "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/antinvestor/service-notification/service/repository/models"
	partapi "github.com/antinvestor/service-partition-api"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"log"
	"time"
)

const defaultLanguageCode = "en"

type notificationBusiness struct {
	service     *frame.Service
	profileCli  *papi.ProfileClient
	partitionCl *partapi.PartitionClient
}

func (nb *notificationBusiness) getNotificationRepo(ctx context.Context) repository.NotificationRepository {
	return repository.NewNotificationRepository(ctx, nb.service)

}

func (nb *notificationBusiness) getTemplateRepo(ctx context.Context) repository.TemplateRepository {
	return repository.NewTemplateRepository(ctx, nb.service)
}

func (nb *notificationBusiness) getLanguageRepo(ctx context.Context) repository.LanguageRepository {
	return repository.NewLanguageRepository(ctx, nb.service)
}

func (nb *notificationBusiness) getChannelRepo(ctx context.Context) repository.ChannelRepository {
	return repository.NewChannelRepository(ctx, nb.service)
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *napi.Notification) (*napi.StatusResponse, error) {

	err := message.Validate()
	if err != nil {
		return nil, err
	}

	access, err := nb.partitionCl.GetAccessById(ctx, message.GetAccessID())
	if err != nil {
		return nil, err
	}

	if access == nil {
		return nil, errors.New("access specified is invalid")
	}

	partition := access.GetPartition()

	var releaseDate time.Time
	if message.AutoRelease {
		releaseDate = time.Now()
	}

	languageCode := message.GetLanguage()
	if languageCode == "" {
		languageCode = defaultLanguageCode
	}
	language, err := nb.getLanguageRepo(ctx).GetByCode(languageCode)
	if err != nil {
		return nil, err
	}

	template, err := nb.getTemplateRepo(ctx).GetByNamePartitionIDAndLanguageID(
		message.GetTemplete(), access.GetPartition().GetPartitionId(), language.ID)
	if err != nil {
		return nil, err
	}

	n := models.Notification{

		AccessID: message.GetAccessID(),
		BaseModel: frame.BaseModel{
			TenantID:    partition.TenantId,
			PartitionID: partition.PartitionId,
		},
		ContactID: message.GetContactID(),

		LanguageID: language.GetID(),
		OutBound:   true,

		TemplateID: template.GetID(),
		Payload:    frame.DBPropertiesFromMap(message.Payload),
		Message:    message.GetData(),

		Type:       message.GetType(),
		State:      int32(common.STATE_CREATED.Number()),
		Status:     int32(common.STATUS_QUEUED.Number()),
		ReleasedAt: &releaseDate,
	}

	n.GenID(ctx)

	// Queue out message for further processing
	err = nb.service.Publish(ctx, config.QueueMessageOutLoggedName, n)
	if err != nil {
		log.Printf("Could not subscriptions message with id : %s - > %v", n.ID, err)
		return nil, err
	}

	status := napi.StatusResponse{ID: n.ID, State: common.STATE(n.State), Status: common.STATUS(n.Status)}

	return &status, nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *napi.Notification) (*napi.StatusResponse, error) {

	err := message.Validate()
	if err != nil {
		return nil, err
	}

	access, err := nb.partitionCl.GetAccessById(ctx, message.GetAccessID())
	if err != nil {
		return nil, err
	}

	if access == nil {
		return nil, errors.New("access specified is invalid")
	}

	partition := access.GetPartition()

	releaseDate := time.Now()

	n := models.Notification{

		AccessID: message.GetAccessID(),
		BaseModel: frame.BaseModel{
			TenantID:    partition.TenantId,
			PartitionID: partition.PartitionId,
		},

		ContactID: message.GetContactID(),

		ChannelID: message.GetRouteID(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:    frame.DBPropertiesFromMap(message.GetPayload()),
		Message:    message.GetData(),
		Type:       message.GetType(),
		ReleasedAt: &releaseDate,
		ExternalID: message.GetID(),

		State:  int32(common.STATE_CREATED),
		Status: int32(common.STATUS_QUEUED),
	}

	// Queue in message for further processing
	err = nb.service.Publish(ctx, config.QueueMessageInLoggedName, n)
	if err != nil {
		log.Printf("Could not queue message %s : -> %+v", n.ID, err)
		return nil, err
	}


	status := napi.StatusResponse{
		ID: n.ID,
		State: common.STATE(n.State),
		Status: common.STATUS(n.Status),
		ExternalID: n.ExternalID}

	return &status, nil
}

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *napi.StatusRequest) (*napi.StatusResponse, error) {

	err := statusReq.Validate()
	if err != nil {
		return nil, err
	}

	n, err := nb.getNotificationRepo(ctx).GetByIDAndProductID(statusReq.GetID(), productID)
	if err != nil {
		return nil, err
	}

	status := napi.StatusResponse{
		NotificationID: n.ID,
		State:          n.State,
		TransientID:    n.TransientID,
		ExternalID:     n.ExternalID,
		ExternalStatus: n.Extra,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *napi.ReleaseRequest) (*napi.StatusResponse, error) {




	n, err := nb.getNotificationRepo(ctx).GetByIDAndProductID(releaseReq., productID)
	if err != nil {
		return nil, err
	}

	status := napi.StatusResponse{
		NotificationID: n.ID,
		State:          n.State,
		Released:       n.IsReleased(),
	}

	return &status, nil
}

func (nb *notificationBusiness) Search(search *napi.SearchRequest, stream napi.NotificationService_SearchServer) error {

	notificationList, err := nb.getNotificationRepo(stream.Context()).Search(search.GetID(), productID)
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
		result := napi.SearchResponse{
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
			log.Printf(" Search -- unable to send a result see %v", err)
		}
	}

	return nil
}
