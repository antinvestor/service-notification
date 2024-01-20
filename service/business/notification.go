package business

import (
	"context"
	"errors"
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
)

const defaultLanguageCode = "en"

type notificationBusiness struct {
	service      *frame.Service
	profileCli   *profileV1.ProfileClient
	partitionCli *partitionV1.PartitionClient
}

func (nb *notificationBusiness) getPartitionData(ctx context.Context, accessID string) (frame.BaseModel, error) {
	if accessID == "" {
		authClaims := frame.ClaimsFromContext(ctx)
		if authClaims != nil {
			return frame.BaseModel{
				TenantID:    authClaims.TenantId(),
				PartitionID: authClaims.PartitionId(),
				AccessID:    authClaims.AccessId(),
			}, nil
		}

		return frame.BaseModel{}, errors.New("access id is empty")
	}

	access, err := nb.partitionCli.GetAccessById(ctx, accessID)
	if err != nil {
		return frame.BaseModel{}, err
	}

	if access == nil {
		return frame.BaseModel{}, errors.New("access specified is invalid")
	}

	partition := access.GetPartition()

	return frame.BaseModel{
		TenantID:    partition.GetTenantId(),
		PartitionID: partition.GetId(),
		AccessID:    accessID,
	}, nil
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := logrus.WithField("request", message)
	logger.Info("handling queue out request")

	partition, err := nb.getPartitionData(ctx, message.GetAccessId())
	if err != nil {
		logger.WithError(err).Warn("could not get partition data")
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
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	profileID := ""
	contactID := message.GetContactId()
	contactData := message.GetDetail()
	if contactData != "" {
		profile, err := nb.profileCli.GetProfileByContact(ctx, contactData)
		if err != nil {
			logger.WithError(err).Warn("could not obtain contact")

			profile, err = nb.profileCli.CreateProfileByContactAndName(ctx, contactData, "")
			if err != nil {
				logger.WithError(err).Warn("could not create contact")
				return nil, err
			}

			//return nil, err
		}

		profileID = profile.GetId()
		for _, contact := range profile.GetContacts() {
			if contact.GetDetail() == contactData {
				contactID = contact.GetId()
				break
			}
		}

	}

	if err != nil {
		logger.WithError(err).Warn("could not get/match contact")
		return nil, err
	}
	n := models.Notification{

		TransientID: message.GetId(),
		BaseModel:   partition,
		ContactID:   contactID,
		ProfileID:   profileID,

		LanguageID: language.GetID(),
		OutBound:   true,

		Payload: frame.DBPropertiesFromMap(message.Payload),
		Message: message.GetData(),

		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
		Priority:         int32(message.GetPriority()),
	}

	if message.GetTemplate() != "" {
		templateRepo := repository.NewTemplateRepository(ctx, nb.service)
		template, err := templateRepo.GetByNamePartitionIDAndLanguageID(
			message.GetTemplate(), partition.PartitionID, language.ID)
		if err != nil {
			logger.WithError(err).Warn("could not get template")
			return nil, err
		}

		n.TemplateID = template.GetID()

	}

	if n.ValidXID(message.GetId()) {
		n.ID = message.GetId()
	} else {
		n.GenID(ctx)
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_CREATED.Number()),
		Status:         int32(commonv1.STATUS_QUEUED.Number()),
	}

	nStatus.GenID(ctx)

	// Queue out message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		logger.WithError(err).Warn("could not emit event save")
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := logrus.WithField("request", message)
	logger.Info("handling queue in request")

	partition, err := nb.getPartitionData(ctx, message.GetAccessId())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
		return nil, err
	}

	releaseDate := time.Now()

	n := models.Notification{

		TransientID: message.GetId(),
		BaseModel:   partition,

		ContactID: message.GetContactId(),

		RouteID: message.GetRouteId(),

		LanguageID: message.GetLanguage(),
		OutBound:   false,

		Payload:          frame.DBPropertiesFromMap(message.GetPayload()),
		Message:          message.GetData(),
		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
		Priority:         int32(message.GetPriority()),
	}

	if n.ValidXID(message.GetId()) {
		n.ID = message.GetId()
	} else {
		n.GenID(ctx)
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_CREATED.Number()),
		Status:         int32(commonv1.STATUS_UNKNOWN.Number()),
	}
	nStatus.GenID(ctx)

	// Queue in message for further processing
	event := events.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification")
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *commonv1.StatusRequest) (*commonv1.StatusResponse, error) {
	logger := logrus.WithField("request", statusReq)
	logger.Info("handling status check request")

	partition, err := nb.getPartitionData(ctx, statusReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not get by id")
		return nil, err
	}

	notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
	nStatus, err := notificationStatusRepo.GetByID(n.StatusID)
	if err != nil {
		logger.WithError(err).Warn("unable to get by status id")
		return nil, err
	}
	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error) {
	logger := logrus.WithField("request", statusReq)
	logger.Info("handling status update request")

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessId())
	if err != nil {
		logger.WithError(err).Warn("could not get access partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not get by id")
		return nil, err
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(statusReq.GetState()),
		Status:         int32(statusReq.GetStatus()),
		ExternalID:     statusReq.GetExternalId(),
		Extra:          frame.DBPropertiesFromMap(statusReq.GetExtras()),
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	eventStatus := events.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationV1.ReleaseRequest) (*commonv1.StatusResponse, error) {

	logger := logrus.WithField("request", releaseReq)
	logger.Info("handling release request")

	partition, err := nb.getPartitionData(ctx, releaseReq.GetAccessId())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, releaseReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not fetch by id")
		return nil, err
	}

	if !n.IsReleased() {
		releaseDate := time.Now()
		n.ReleasedAt = &releaseDate

		event := events.NotificationSave{}
		err = nb.service.Emit(ctx, event.Name(), n)
		if err != nil {
			logger.WithError(err).Warn("could not emit notification save")
			return nil, err
		}

		nStatus := models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_ACTIVE.Number()),
			Status:         int32(commonv1.STATUS_QUEUED.Number()),
		}

		nStatus.GenID(ctx)

		// Release notification status save for further processing
		eventStatus := events.NotificationStatusSave{}
		err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
		if err != nil {
			logger.WithError(err).Warn("could not emit notification status")
			return nil, err
		}

		return nStatus.ToStatusAPI(), nil
	} else {

		notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
		nStatus, err := notificationStatusRepo.GetByID(n.StatusID)
		if err != nil {
			logger.WithError(err).Warn("could not get notification status")
			return nil, err
		}

		return nStatus.ToStatusAPI(), nil
	}
}

func (nb *notificationBusiness) Search(search *commonv1.SearchRequest,
	stream notificationV1.NotificationService_SearchServer) error {
	logger := logrus.WithField("request", search)

	logger.Info("handling search request")

	ctx := stream.Context()
	authClaims := frame.ClaimsFromContext(ctx)

	partitionId := ""
	if authClaims != nil {
		partitionId = authClaims.PartitionId()
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)

	query := search.GetIdQuery()
	if query == "" {
		query = search.GetQuery()
	}
	notificationList, err := notificationRepo.SearchByPartition(partitionId, query)
	if err != nil {
		logger.WithError(err).Warn("failed to search notifications")
		return err
	}

	var responsesList []*notificationV1.Notification
	for _, n := range notificationList {
		nStatus := &models.NotificationStatus{}
		if n.StatusID != "" {
			resultStatus, err := notificationStatusRepo.GetByID(n.StatusID)
			if err != nil {
				logger.WithError(err).WithField("status_id", n.StatusID).Warn(" could not get status id for")
			} else {
				nStatus = resultStatus
			}
		}

		result := n.ToNotificationApi(nStatus)
		responsesList = append(responsesList, result)
	}

	err = stream.Send(&notificationV1.SearchResponse{Data: responsesList})
	if err != nil {
		logger.WithError(err).Warn(" unable to send a result")
	}

	logger.Info("_______________________________________________________")
	logger.WithField("result count", len(responsesList)).
		Infof("_____  Sending out %d object   _______________", len(notificationList))
	logger.Info("_______________________________________________________")

	return nil
}
