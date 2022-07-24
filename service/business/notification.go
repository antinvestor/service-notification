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

	languageRepo := repository.NewLanguageRepository(ctx, nb.service)
	language, err := languageRepo.GetByCode(languageCode)
	if err != nil {
		return nil, err
	}

	n := models.Notification{

		TransientID: message.GetID(),
		BaseModel:   partition,
		ContactID:   message.GetContactID(),

		LanguageID: language.GetID(),
		OutBound:   true,

		Payload: frame.DBPropertiesFromMap(message.Payload),
		Message: message.GetData(),

		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
	}

	if message.GetTemplete() != "" {
		templateRepo := repository.NewTemplateRepository(ctx, nb.service)
		template, err := templateRepo.GetByNamePartitionIDAndLanguageID(
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

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(common.STATE_CREATED.Number()),
		Status:         int32(common.STATUS_QUEUED.Number()),
	}

	nStatus.GenID(ctx)

	// Queue out message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		return nil, err
	}

	return nStatus.ToStatusApi(), nil
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

		TransientID: message.GetID(),
		BaseModel:   partition,

		ContactID: message.GetContactID(),

		RouteID: message.GetRouteID(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:          frame.DBPropertiesFromMap(message.GetPayload()),
		Message:          message.GetData(),
		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
	}

	if n.ValidXID(message.GetID()) {
		n.ID = message.GetID()
	} else {
		n.GenID(ctx)
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(common.STATE_CREATED.Number()),
		Status:         int32(common.STATUS_QUEUED.Number()),
	}
	nStatus.GenID(ctx)

	// Queue in message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		return nil, err
	}

	return nStatus.ToStatusApi(), nil
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

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		return nil, err
	}

	notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
	nStatus, err := notificationStatusRepo.GetByID(n.StatusID)
	if err != nil {
		return nil, err
	}
	return nStatus.ToStatusApi(), nil
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

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		return nil, err
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(statusReq.GetState()),
		Status:         int32(statusReq.GetStatus()),
		ExternalID:     statusReq.GetExternalID(),
		Extra:          frame.DBPropertiesFromMap(statusReq.GetExtras()),
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		return nil, err
	}

	return nStatus.ToStatusApi(), nil
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

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, releaseReq.GetID())
	if err != nil {
		return nil, err
	}

	if !n.IsReleased() {
		releaseDate := time.Now()
		n.ReleasedAt = &releaseDate

		event := events.NotificationSave{}
		err = nb.service.Emit(ctx, event.Name(), n)
		if err != nil {
			return nil, err
		}

		nStatus := models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(common.STATE_ACTIVE.Number()),
			Status:         int32(common.STATUS_QUEUED.Number()),
		}

		nStatus.GenID(ctx)

		// Release notification status save for further processing
		eventStatus := events.NotificationStatusSave{}
		err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
		if err != nil {
			return nil, err
		}

		return nStatus.ToStatusApi(), nil
	} else {

		notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
		nStatus, err := notificationStatusRepo.GetByID(n.StatusID)
		if err != nil {
			return nil, err
		}

		return nStatus.ToStatusApi(), nil
	}
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

	notificationRepo := repository.NewNotificationRepository(stream.Context(), nb.service)

	notificationStatusRepo := repository.NewNotificationStatusRepository(stream.Context(), nb.service)

	notificationList, err := notificationRepo.SearchByPartition(partition.PartitionID, search.GetQuery())
	if err != nil {
		return err
	}

	for _, n := range notificationList {
		nStatus, err := notificationStatusRepo.GetByID(n.StatusID)
		if err != nil {
			return err
		}

		result := n.ToNotificationApi(nStatus)
		err = stream.Send(result)
		if err != nil {
			nb.service.L().Info(" Search -- unable to send a result see %v", err)
		}
	}

	return nil
}
