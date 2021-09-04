package business

import (
	"context"
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

func (nb *notificationBusiness) getPartitionData(ctx context.Context, accessId string) (frame.BaseModel, error) {

	access, err := nb.partitionCl.GetAccessById(ctx, accessId)
	if err != nil {
		return frame.BaseModel{}, err
	}

	if access == nil {
		return frame.BaseModel{}, errors.New("access specified is invalid")
	}

	partition := access.GetPartition()

	return frame.BaseModel{
		TenantID:    partition.TenantId,
		PartitionID: partition.PartitionId,
		AccessID: accessId,
	}, nil
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *napi.Notification) (*napi.StatusResponse, error) {

	err := message.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())

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
		message.GetTemplete(), partition.PartitionID, language.ID)
	if err != nil {
		return nil, err
	}

	n := models.Notification{

		AccessID: message.GetAccessID(),
		BaseModel: partition,
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

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())

	releaseDate := time.Now()

	n := models.Notification{

		AccessID: message.GetAccessID(),
		BaseModel: partition,

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

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessID())

	n, err := nb.getNotificationRepo(ctx).GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		return nil, err
	}

	return n.ToStatusApi(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *napi.ReleaseRequest) (*napi.StatusResponse, error) {


	err := releaseReq.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, releaseReq.GetAccessID())

	n, err := nb.getNotificationRepo(ctx).GetByPartitionAndID(partition.PartitionID, releaseReq.GetID())
	if err != nil {
		return nil, err
	}

	if  !n.IsReleased() {
		releaseDate := time.Now()
		n.ReleasedAt = &releaseDate
	}

	return n.ToStatusApi(), nil

}

func (nb *notificationBusiness) Search(search *napi.SearchRequest, stream napi.NotificationService_SearchServer) error {

	err := search.Validate()
	if err != nil {
		return err
	}

	partition, err := nb.getPartitionData(stream.Context(), search.GetAccessID())

	notificationList, err := nb.getNotificationRepo(stream.Context()).SearchByPartition(partition.PartitionID, search.GetQuery())
	if err != nil {
		return err
	}

	for _, n := range notificationList {

		result := n.ToNotificationApi()
		err = stream.Send(result)
		if err != nil {
			log.Printf(" Search -- unable to send a result see %v", err)
		}
	}

	return nil
}
