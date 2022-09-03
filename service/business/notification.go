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
				TenantID:    authClaims.TenantID,
				PartitionID: authClaims.PartitionID,
				AccessID:    authClaims.AccessID,
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
		TenantID:    partition.TenantId,
		PartitionID: partition.PartitionId,
		AccessID:    accessID,
	}, nil
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationV1.Notification) (*notificationV1.StatusResponse, error) {
	logger := logrus.WithField("request", message)
	logger.Info("handling queue out request")

	err := message.Validate()
	if err != nil {
		logger.WithError(err).Warn("failed validation")
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())
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
	contactID := message.GetContactID()
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

		profileID = profile.GetID()
		for _, contact := range profile.GetContacts() {
			if contact.GetDetail() == contactData {
				contactID = contact.GetID()
				break
			}
		}

	}

	if err != nil {
		logger.WithError(err).Warn("could not get/match contact")
		return nil, err
	}
	n := models.Notification{

		TransientID: message.GetID(),
		BaseModel:   partition,
		ContactID:   contactID,
		ProfileID:   profileID,

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
			logger.WithError(err).Warn("could not get template")
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

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationV1.Notification) (*notificationV1.StatusResponse, error) {
	logger := logrus.WithField("request", message)
	logger.Info("handling queue in request")

	err := message.Validate()
	if err != nil {
		logger.WithError(err).Warn("failed validation")
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, message.GetAccessID())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
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

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *notificationV1.StatusRequest) (*notificationV1.StatusResponse, error) {
	logger := logrus.WithField("request", statusReq)
	logger.Info("handling status check request")

	err := statusReq.Validate()
	if err != nil {
		logger.WithError(err).Warn("failed to validate request")
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessID())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
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

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *notificationV1.StatusUpdateRequest) (*notificationV1.StatusResponse, error) {
	logger := logrus.WithField("request", statusReq)
	logger.Info("handling status update request")

	err := statusReq.Validate()
	if err != nil {
		logger.WithError(err).Warn("could not validate request")
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, statusReq.GetAccessID())
	if err != nil {
		logger.WithError(err).Warn("could not get access partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, statusReq.GetID())
	if err != nil {
		logger.WithError(err).Warn("could not get by id")
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
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationV1.ReleaseRequest) (*notificationV1.StatusResponse, error) {

	logger := logrus.WithField("request", releaseReq)
	logger.Info("handling release request")

	err := releaseReq.Validate()
	if err != nil {
		logger.WithError(err).Warn("failed request validation")
		return nil, err
	}

	partition, err := nb.getPartitionData(ctx, releaseReq.GetAccessID())
	if err != nil {
		logger.WithError(err).Warn("could not get partition")
		return nil, err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByPartitionAndID(partition.PartitionID, releaseReq.GetID())
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
			State:          int32(common.STATE_ACTIVE.Number()),
			Status:         int32(common.STATUS_QUEUED.Number()),
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

func (nb *notificationBusiness) Search(search *notificationV1.SearchRequest,
	stream notificationV1.NotificationService_SearchServer) error {
	logger := logrus.WithField("request", search)

	logger.Info("handling search request")

	err := search.Validate()
	if err != nil {
		logger.WithError(err).Warn("failed request validation")
		return err
	}

	partition, err := nb.getPartitionData(stream.Context(), search.GetAccessID())
	if err != nil {
		logger.WithError(err).Warn("failed to obtain partition data")
		return err
	}

	notificationRepo := repository.NewNotificationRepository(stream.Context(), nb.service)

	notificationStatusRepo := repository.NewNotificationStatusRepository(stream.Context(), nb.service)

	notificationList, err := notificationRepo.SearchByPartition(partition.PartitionID, search.GetQuery())
	if err != nil {
		logger.WithError(err).Warn("failed to search notifications")
		return err
	}

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
		err = stream.Send(result)
		if err != nil {
			logger.WithError(err).Warn(" unable to send a result")
		}
	}

	logger.Info("_______________________________________________________")
	logger.WithField("result count", len(notificationList)).
		Infof("_____  Sending out %d object   _______________", len(notificationList))
	logger.Info("_______________________________________________________")

	return nil
}
