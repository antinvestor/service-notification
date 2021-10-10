package business

import (
	"context"
	"errors"
	"time"

	"github.com/antinvestor/apis/common"
	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	partitionV1 "github.com/antinvestor/service-partition-api"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

const defaultLanguageCode = "en"

type notificationBusiness struct {
	service      *frame.Service
	profileCli   *profileV1.ProfileClient
	partitionCli *partitionV1.PartitionClient
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

func (nb *notificationBusiness) getChannelRepo(ctx context.Context) repository.RouteRepository {
	return repository.NewRouteRepository(ctx, nb.service)
}

func (nb *notificationBusiness) getPartitionData(ctx context.Context, accessId string) (frame.BaseModel, error) {

	access, err := nb.partitionCli.GetAccessById(ctx, accessId)
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
		AccessID:    accessId,
	}, nil
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationV1.Notification) (*notificationV1.StatusResponse, error) {

	err := message.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())
	if err != nil {
		return nil, err
	}

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

	n := models.Notification{

		TransientID: message.GetID(),
		AccessID:    message.GetAccessID(),
		BaseModel:   partition,
		ContactID:   message.GetContactID(),

		LanguageID: language.GetID(),
		OutBound:   true,

		Payload: frame.DBPropertiesFromMap(message.Payload),
		Message: message.GetData(),

		NotificationType: message.GetType(),
		State:            int32(common.STATE_CREATED.Number()),
		Status:           int32(common.STATUS_QUEUED.Number()),
		ReleasedAt:       &releaseDate,
	}

	if message.GetTemplete() != "" {
		template, err := nb.getTemplateRepo(ctx).GetByNamePartitionIDAndLanguageID(
			message.GetTemplete(), partition.PartitionID, language.ID)
		if err != nil {
			return nil, err
		}

		n.TemplateID = template.GetID()

	}

	if n.ValidXID(message.GetID()) {
		n.ID = message.GetID()
	} else {
		n.GenID(ctx)
	}

	// Queue out message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		return nil, err
	}

	return n.ToStatusApi(), nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationV1.Notification) (*notificationV1.StatusResponse, error) {

	err := message.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())
	if err != nil {
		return nil, err
	}

	releaseDate := time.Now()

	n := models.Notification{

		ExternalID: message.GetID(),
		AccessID:   message.GetAccessID(),
		BaseModel:  partition,

		ContactID: message.GetContactID(),

		RouteID: message.GetRouteID(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:          frame.DBPropertiesFromMap(message.GetPayload()),
		Message:          message.GetData(),
		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,

		State:  int32(common.STATE_CREATED),
		Status: int32(common.STATUS_QUEUED),
	}

	if n.ValidXID(message.GetID()) {
		n.ID = message.GetID()
	} else {
		n.GenID(ctx)
	}

	// Queue in message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		return nil, err
	}

	return n.ToStatusApi(), nil
}

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *notificationV1.StatusRequest) (*notificationV1.StatusResponse, error) {

	err := statusReq.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessID())
	if err != nil {
		return nil, err
	}

	n, err := nb.getNotificationRepo(ctx).GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		return nil, err
	}

	return n.ToStatusApi(), nil
}

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *notificationV1.StatusUpdateRequest) (*notificationV1.StatusResponse, error) {

	err := statusReq.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessID())
	if err != nil {
		return nil, err
	}

	n, err := nb.getNotificationRepo(ctx).GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		return nil, err
	}

	if statusReq.GetState() != common.STATE_CREATED {
		n.State = int32(statusReq.GetState())
	}

	if statusReq.GetStatus() != common.STATUS_UNKNOWN {
		n.Status = int32(statusReq.GetStatus())
	}

	if statusReq.GetExternalID() != "" {
		n.ExternalID = statusReq.GetExternalID()
	}

	if len(statusReq.GetExtras()) != 0 {

		dbMap := frame.DBPropertiesToMap(n.Extra)

		for k, v := range statusReq.GetExtras() {
			dbMap[k] = v
		}

		n.Extra = frame.DBPropertiesFromMap(dbMap)
	}

	//TODO: create a ledger of status changes
	err = nb.getNotificationRepo(ctx).Save(n)
	if err != nil {
		return nil, err
	}

	return n.ToStatusApi(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationV1.ReleaseRequest) (*notificationV1.StatusResponse, error) {

	err := releaseReq.Validate()
	if err != nil {
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, releaseReq.GetAccessID())
	if err != nil {
		return nil, err
	}

	n, err := nb.getNotificationRepo(ctx).GetByPartitionAndID(partition.PartitionID, releaseReq.GetID())
	if err != nil {
		return nil, err
	}

	if !n.IsReleased() {
		releaseDate := time.Now()
		n.ReleasedAt = &releaseDate
	}

	return n.ToStatusApi(), nil

}

func (nb *notificationBusiness) Search(search *notificationV1.SearchRequest, stream notificationV1.NotificationService_SearchServer) error {

	err := search.Validate()
	if err != nil {
		return err
	}

	partition, err := nb.getPartitionData(stream.Context(), search.GetAccessID())
	if err != nil {
		return err
	}

	notificationList, err := nb.getNotificationRepo(stream.Context()).SearchByPartition(partition.PartitionID, search.GetQuery())
	if err != nil {
		return err
	}

	for _, n := range notificationList {

		result := n.ToNotificationApi()
		err = stream.Send(result)
		if err != nil {
			nb.service.L().Info(" Search -- unable to send a result see %v", err)
		}
	}

	return nil
}
