package business

import (
	"context"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	repository2 "github.com/antinvestor/service-notification/apps/default/service/repository"
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/pitabwire/frame"
	"google.golang.org/grpc"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *notificationV1.Notification) (*commonv1.StatusResponse, error)
	QueueIn(ctx context.Context, in *notificationV1.Notification) (*commonv1.StatusResponse, error)
	Status(ctx context.Context, status *commonv1.StatusRequest) (*commonv1.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error)
	Release(ctx context.Context, req *notificationV1.ReleaseRequest, stream grpc.ServerStreamingServer[notificationV1.ReleaseResponse]) error
	Search(search *commonv1.SearchRequest, stream grpc.ServerStreamingServer[notificationV1.SearchResponse]) error
	TemplateSave(ctx context.Context, req *notificationV1.TemplateSaveRequest) (*notificationV1.Template, error)
	TemplateSearch(search *notificationV1.TemplateSearchRequest, stream grpc.ServerStreamingServer[notificationV1.TemplateSearchResponse]) error
}

func NewNotificationBusiness(_ context.Context, service *frame.Service, profileCli *profileV1.ProfileClient, partitionCli *partitionV1.PartitionClient) (NotificationBusiness, error) {

	if service == nil || profileCli == nil || partitionCli == nil {
		return nil, ErrorInitializationFail
	}

	return &notificationBusiness{
		service:      service,
		profileCli:   profileCli,
		partitionCli: partitionCli,
	}, nil
}

type notificationBusiness struct {
	service      *frame.Service
	profileCli   *profileV1.ProfileClient
	partitionCli *partitionV1.PartitionClient
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", message)

	authClaim := frame.ClaimsFromContext(ctx)

	logger.WithField("auth claim", authClaim).Info("handling queue out request")

	var releaseDate time.Time
	if message.AutoRelease {
		releaseDate = time.Now()
	}

	languageRepo := repository2.NewLanguageRepository(ctx, nb.service)
	language, err := languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	n := models.Notification{
		ParentID:          message.GetParentId(),
		TransientID:       message.GetId(),
		SenderProfileType: message.GetSource().GetProfileType(),
		SenderProfileID:   message.GetSource().GetProfileId(),
		SenderContactID:   message.GetSource().GetContactId(),

		RecipientProfileType: message.GetRecipient().GetProfileType(),
		RecipientProfileID:   message.GetRecipient().GetProfileId(),
		RecipientContactID:   message.GetRecipient().GetContactId(),

		LanguageID: language.GetID(),
		OutBound:   true,

		Payload: frame.DBPropertiesFromMap(message.Payload),
		Message: message.GetData(),

		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
		Priority:         int32(message.GetPriority()),
	}

	n.GenID(ctx)
	if n.ValidXID(message.GetId()) {
		n.ID = message.GetId()
	}

	if message.GetTemplate() != "" {
		templateRepo := repository2.NewTemplateRepository(ctx, nb.service)
		t, err0 := templateRepo.GetByName(ctx, message.GetTemplate())
		if err0 != nil {
			logger.WithError(err0).Warn("could not get template")
			return nil, err0
		}

		n.TemplateID = t.GetID()
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_CREATED.Number()),
		Status:         int32(commonv1.STATUS_QUEUED.Number()),
	}

	nStatus.GenID(ctx)

	// Queue out message for further processing
	event := events2.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		logger.WithError(err).Warn("could not emit event save")
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events2.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", message)

	authClaim := frame.ClaimsFromContext(ctx)
	logger.WithField("auth claim", authClaim).Info("handling queue in request")

	releaseDate := time.Now()

	languageRepo := repository2.NewLanguageRepository(ctx, nb.service)
	language, err := languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	n := models.Notification{
		ParentID:          message.GetParentId(),
		TransientID:       message.GetId(),
		SenderProfileType: message.GetSource().GetProfileType(),
		SenderProfileID:   message.GetSource().GetProfileId(),
		SenderContactID:   message.GetSource().GetContactId(),

		RecipientProfileType: message.GetRecipient().GetProfileType(),
		RecipientProfileID:   message.GetRecipient().GetProfileId(),
		RecipientContactID:   message.GetRecipient().GetContactId(),
		RouteID:              message.GetRouteId(),
		LanguageID:           language.GetID(),
		OutBound:             false,

		Payload:          frame.DBPropertiesFromMap(message.GetPayload()),
		Message:          message.GetData(),
		NotificationType: message.GetType(),
		ReleasedAt:       &releaseDate,
		Priority:         int32(message.GetPriority()),
	}

	n.GenID(ctx)
	if n.ValidXID(message.GetId()) {
		n.ID = message.GetId()
	}

	nStatus := models.NotificationStatus{
		NotificationID: n.GetID(),
		State:          int32(commonv1.STATE_CREATED.Number()),
		Status:         int32(commonv1.STATUS_UNKNOWN.Number()),
	}
	nStatus.GenID(ctx)

	// Queue in message for further processing
	event := events2.NotificationSave{}
	err = nb.service.Emit(ctx, event.Name(), n)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification")
		return nil, err
	}

	// Queue out notification status for further processing
	eventStatus := events2.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *commonv1.StatusRequest) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", statusReq)
	logger.Info("handling status check request")

	notificationRepo := repository2.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByID(ctx, statusReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not get by id")
		return nil, err
	}

	notificationStatusRepo := repository2.NewNotificationStatusRepository(ctx, nb.service)
	nStatus, err := notificationStatusRepo.GetByID(ctx, n.StatusID)
	if err != nil {
		logger.WithError(err).Warn("unable to get by status id")
		return nil, err
	}
	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", statusReq)
	logger.Debug("handling status update request")

	notificationRepo := repository2.NewNotificationRepository(ctx, nb.service)

	n, err := notificationRepo.GetByID(ctx, statusReq.GetId())
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
	eventStatus := events2.NotificationStatusSave{}
	err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationV1.ReleaseRequest, stream grpc.ServerStreamingServer[notificationV1.ReleaseResponse]) error {

	logger := nb.service.Log(ctx).WithField("request", releaseReq)
	logger.Debug("handling release request")

	notificationRepo := repository2.NewNotificationRepository(ctx, nb.service)
	notificationList, err := notificationRepo.GetByIDList(ctx, releaseReq.GetId()...)
	if err != nil {
		logger.WithError(err).Warn("could not fetch by id")
		return err
	}

	var releasedStatusIDs []string
	var notificationsToUpdate []*models.Notification

	releaseDate := time.Now()

	for _, n := range notificationList {

		if n.IsReleased() {
			releasedStatusIDs = append(releasedStatusIDs, n.StatusID)
		} else {
			n.ReleasedAt = &releaseDate
			notificationsToUpdate = append(notificationsToUpdate, n)
		}

	}

	var statusesToRelease []*commonv1.StatusResponse
	for _, n := range notificationsToUpdate {

		event := events2.NotificationSave{}
		err = nb.service.Emit(ctx, event.Name(), n)
		if err != nil {
			logger.WithError(err).Warn("could not emit notification save")
			return err
		}

		nStatus := models.NotificationStatus{
			NotificationID: n.GetID(),
			State:          int32(commonv1.STATE_ACTIVE.Number()),
			Status:         int32(commonv1.STATUS_QUEUED.Number()),
		}

		nStatus.GenID(ctx)

		// Release notification status save for further processing
		eventStatus := events2.NotificationStatusSave{}
		err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
		if err != nil {
			logger.WithError(err).Warn("could not emit notification status")
			return err
		}

		statusesToRelease = append(statusesToRelease, nStatus.ToStatusAPI())
	}

	if len(statusesToRelease) > 0 {
		err = stream.Send(&notificationV1.ReleaseResponse{Data: statusesToRelease})
		if err != nil {
			return err
		}
	}

	statusesToRelease = nil
	var notificationStatusList []*models.NotificationStatus
	if len(releasedStatusIDs) > 0 {
		notificationStatusRepo := repository2.NewNotificationStatusRepository(ctx, nb.service)
		notificationStatusList, err = notificationStatusRepo.GetByIDList(ctx, releasedStatusIDs...)
		if err != nil {
			logger.WithError(err).Warn("could not get notification status")
			return err
		}

		for _, nStatus := range notificationStatusList {
			statusesToRelease = append(statusesToRelease, nStatus.ToStatusAPI())
		}
	}

	if len(statusesToRelease) > 0 {
		err = stream.Send(&notificationV1.ReleaseResponse{Data: statusesToRelease})
		if err != nil {
			return err
		}
	}

	return nil
}

func (nb *notificationBusiness) Search(search *commonv1.SearchRequest,
	stream notificationV1.NotificationService_SearchServer) error {

	ctx := stream.Context()

	logger := nb.service.Log(ctx).WithField("request", search)

	logger.Debug("handling search request")

	jwtToken := frame.JwtFromContext(ctx)

	logger.WithField("jwt", jwtToken).Debug("auth jwt supplied")

	var notificationList []*models.Notification
	var err error

	notificationRepo := repository2.NewNotificationRepository(ctx, nb.service)

	if search.GetIdQuery() != "" {
		notification, err0 := notificationRepo.GetByID(ctx, search.GetIdQuery())
		if err0 != nil {
			return err0
		}

		notificationList = append(notificationList, notification)

	} else {

		notificationList, err = notificationRepo.Search(ctx, search.GetQuery())
		if err != nil {
			logger.WithError(err).Error("failed to search notifications")
			return err
		}
	}

	notificationStatusRepo := repository2.NewNotificationStatusRepository(ctx, nb.service)
	languageRepo := repository2.NewLanguageRepository(ctx, nb.service)

	var resultStatus *models.NotificationStatus
	var language *models.Language
	var responsesList []*notificationV1.Notification
	for _, n := range notificationList {
		nStatus := &models.NotificationStatus{}
		if n.StatusID != "" {
			resultStatus, err = notificationStatusRepo.GetByID(ctx, n.StatusID)
			if err != nil {
				logger.WithError(err).WithField("status_id", n.StatusID).Error(" could not get status id for")
				return err
			} else {
				nStatus = resultStatus
			}
		}

		language, err = languageRepo.GetByID(ctx, n.LanguageID)
		if err != nil {
			logger.WithError(err).WithField("language_id", n.LanguageID).Error(" could not get language id")
			return err
		}

		result := n.ToApi(nStatus, language, nil)
		responsesList = append(responsesList, result)
	}

	err = stream.Send(&notificationV1.SearchResponse{Data: responsesList})
	if err != nil {
		logger.WithError(err).Warn(" unable to send a result")
	}

	return nil
}

func (nb *notificationBusiness) TemplateSearch(search *notificationV1.TemplateSearchRequest,
	stream notificationV1.NotificationService_TemplateSearchServer) error {

	ctx := stream.Context()
	logger := nb.service.Log(ctx).WithField("request", search)

	queryString := search.GetQuery()

	authClaims := frame.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Debug("handling template search request")

	templateRepository := repository2.NewTemplateRepository(ctx, nb.service)
	templateList, err := templateRepository.SearchByName(ctx, queryString, int(search.GetPage()), int(search.GetCount()))
	if err != nil {
		return err
	}

	languageRepo := repository2.NewLanguageRepository(ctx, nb.service)

	var language *models.Language
	if search.GetLanguageCode() != "" {
		language, err = languageRepo.GetOrCreateByCode(ctx, search.GetLanguageCode())

		if err != nil {
			return err
		}
	}

	templateDataRepository := repository2.NewTemplateDataRepository(ctx, nb.service)

	var responseList []*notificationV1.Template
	var templateDataList []*models.TemplateData
	languageMap := map[string]*models.Language{}

	for _, t := range templateList {

		var apiTemplateDataList []*notificationV1.TemplateData

		templateDataList, err = templateDataRepository.GetByTemplateID(ctx, t.GetID())
		if err != nil {
			logger.WithError(err).Warn(" unable to get template data")
			return err
		}

		for _, data := range templateDataList {

			if language != nil && language.GetID() != data.LanguageID {
				continue
			}

			lang, ok := languageMap[data.LanguageID]
			if !ok {

				lang, err = languageRepo.GetByID(ctx, data.LanguageID)
				if err != nil {
					return err
				}
				languageMap[data.LanguageID] = lang
			}

			apiTemplateDataList = append(apiTemplateDataList, data.ToApi(lang.ToApi()))
		}

		result := t.ToApi(apiTemplateDataList)
		responseList = append(responseList, result)
	}

	err = stream.Send(&notificationV1.TemplateSearchResponse{Data: responseList})
	if err != nil {
		logger.WithError(err).Warn(" unable to send a result")
		return err
	}

	return nil
}

func (nb *notificationBusiness) TemplateSave(ctx context.Context, req *notificationV1.TemplateSaveRequest) (*notificationV1.Template, error) {
	logger := nb.service.Log(ctx).WithField("request", req)

	authClaims := frame.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Info("handling template request update")

	languageRepo := repository2.NewLanguageRepository(ctx, nb.service)
	language, err := languageRepo.GetOrCreateByCode(ctx, req.GetLanguageCode())

	if err != nil {
		logger.WithError(err).Debug("language for template is required")
		return nil, err
	}

	template := &models.Template{
		Name:  req.GetName(),
		Extra: frame.DBPropertiesFromMap(req.GetExtra()),
	}

	templateRepository := repository2.NewTemplateRepository(ctx, nb.service)
	err = templateRepository.Save(ctx, template)
	if err != nil {
		return nil, err
	}

	templateDataRepository := repository2.NewTemplateDataRepository(ctx, nb.service)

	for key, val := range req.GetData() {
		templateData := &models.TemplateData{
			TemplateID: template.GetID(),
			LanguageID: language.GetID(),
			Type:       key,
			Detail:     val,
		}

		err = templateDataRepository.Save(ctx, templateData)
		if err != nil {
			return nil, err
		}
	}

	template, err = templateRepository.GetByID(ctx, template.GetID())
	if err != nil {
		logger.WithError(err).Debug("could not get existing template")
		return nil, err
	}

	languageMap := map[string]*models.Language{}

	var apiTemplateDataList []*notificationV1.TemplateData

	templateDataList, err0 := templateDataRepository.GetByTemplateID(ctx, template.GetID())
	if err0 != nil {
		logger.WithError(err0).Debug("could not get existing template data")
		return nil, err
	}
	for _, data := range templateDataList {

		lang, ok := languageMap[data.LanguageID]
		if !ok {

			lang, err = languageRepo.GetByID(ctx, data.LanguageID)
			if err != nil {
				return nil, err
			}
		}

		apiTemplateDataList = append(apiTemplateDataList, data.ToApi(lang.ToApi()))
	}

	return template.ToApi(apiTemplateDataList), nil
}
