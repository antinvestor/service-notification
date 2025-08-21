package business

import (
	"context"
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/framedata"
	"google.golang.org/grpc"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *notificationv1.Notification) (*commonv1.StatusResponse, error)
	QueueIn(ctx context.Context, in *notificationv1.Notification) (*commonv1.StatusResponse, error)
	Status(ctx context.Context, status *commonv1.StatusRequest) (*commonv1.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error)
	Release(ctx context.Context, req *notificationv1.ReleaseRequest, stream grpc.ServerStreamingServer[notificationv1.ReleaseResponse]) error
	Search(search *commonv1.SearchRequest, stream grpc.ServerStreamingServer[notificationv1.SearchResponse]) error
	TemplateSave(ctx context.Context, req *notificationv1.TemplateSaveRequest) (*notificationv1.Template, error)
	TemplateSearch(search *notificationv1.TemplateSearchRequest, stream grpc.ServerStreamingServer[notificationv1.TemplateSearchResponse]) error
}

func NewNotificationBusiness(ctx context.Context, service *frame.Service, profileCli *profilev1.ProfileClient, partitionCli *partitionV1.PartitionClient) (NotificationBusiness, error) {

	if service == nil || profileCli == nil || partitionCli == nil {
		return nil, ErrorInitializationFail
	}

	return &notificationBusiness{
		service:                service,
		profileCli:             profileCli,
		partitionCli:           partitionCli,
		notificationRepo:       repository.NewNotificationRepository(ctx, service),
		notificationStatusRepo: repository.NewNotificationStatusRepository(ctx, service),
		languageRepo:           repository.NewLanguageRepository(ctx, service),
		templateRepo:           repository.NewTemplateRepository(ctx, service),
		templateDataRepo:       repository.NewTemplateDataRepository(ctx, service),
	}, nil
}

type notificationBusiness struct {
	service                *frame.Service
	profileCli             *profilev1.ProfileClient
	partitionCli           *partitionV1.PartitionClient
	notificationRepo       repository.NotificationRepository
	notificationStatusRepo repository.NotificationStatusRepository
	languageRepo           repository.LanguageRepository
	templateRepo           repository.TemplateRepository
	templateDataRepo       repository.TemplateDataRepository
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationv1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", message)

	authClaim := frame.ClaimsFromContext(ctx)

	logger.WithField("auth claim", authClaim).Info("handling queue out request")

	var releaseDate time.Time
	if message.AutoRelease {
		releaseDate = time.Now()
	}

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

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
		t, err0 := nb.templateRepo.GetByName(ctx, message.GetTemplate())
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

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationv1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", message)

	authClaim := frame.ClaimsFromContext(ctx)
	logger.WithField("auth claim", authClaim).Info("handling queue in request")

	releaseDate := time.Now()

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

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
	logger := nb.service.Log(ctx).WithField("request", statusReq)
	logger.Info("handling status check request")

	n, err := nb.notificationRepo.GetByID(ctx, statusReq.GetId())
	if err != nil {
		logger.WithError(err).Warn("could not get by id")
		return nil, err
	}

	nStatus, err := nb.notificationStatusRepo.GetByID(ctx, n.StatusID)
	if err != nil {
		logger.WithError(err).Warn("unable to get by status id")
		return nil, err
	}
	return nStatus.ToStatusAPI(), nil
}

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error) {
	logger := nb.service.Log(ctx).WithField("request", statusReq)
	logger.Debug("handling status update request")

	n, err := nb.notificationRepo.GetByID(ctx, statusReq.GetId())
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

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationv1.ReleaseRequest, stream grpc.ServerStreamingServer[notificationv1.ReleaseResponse]) error {

	logger := nb.service.Log(ctx).WithField("request", releaseReq)
	logger.Debug("handling release request")

	notificationList, err := nb.notificationRepo.GetByIDList(ctx, releaseReq.GetId()...)
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

		event := events.NotificationSave{}
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
		eventStatus := events.NotificationStatusSave{}
		err = nb.service.Emit(ctx, eventStatus.Name(), nStatus)
		if err != nil {
			logger.WithError(err).Warn("could not emit notification status")
			return err
		}

		statusesToRelease = append(statusesToRelease, nStatus.ToStatusAPI())
	}

	if len(statusesToRelease) > 0 {
		err = stream.Send(&notificationv1.ReleaseResponse{Data: statusesToRelease})
		if err != nil {
			return err
		}
	}

	statusesToRelease = nil
	var notificationStatusList []*models.NotificationStatus
	if len(releasedStatusIDs) > 0 {
		notificationStatusList, err = nb.notificationStatusRepo.GetByIDList(ctx, releasedStatusIDs...)
		if err != nil {
			logger.WithError(err).Warn("could not get notification status")
			return err
		}

		for _, nStatus := range notificationStatusList {
			statusesToRelease = append(statusesToRelease, nStatus.ToStatusAPI())
		}
	}

	if len(statusesToRelease) > 0 {
		err = stream.Send(&notificationv1.ReleaseResponse{Data: statusesToRelease})
		if err != nil {
			return err
		}
	}

	return nil
}

func (nb *notificationBusiness) Search(search *commonv1.SearchRequest,
	stream notificationv1.NotificationService_SearchServer) error {

	ctx := stream.Context()

	logger := nb.service.Log(ctx).WithField("request", search)

	logger.Debug("handling search request")

	profileID := ""
	claims := frame.ClaimsFromContext(ctx)
	if claims != nil {
		profileID, _ = claims.GetSubject()
	}

	limits := search.GetLimits()
	if limits == nil {
		limits = &commonv1.Pagination{
			Count:     20,
			Page:      0,
			StartDate: time.Now().Add(-6 * time.Hour).String(),
			EndDate:   time.Now().String(),
		}
	}

	searchProperties := map[string]any{
		"profile_id": profileID,
		"start_date": limits.GetStartDate(),
		"end_date":   limits.GetEndDate(),
	}
	for k, val := range search.GetExtras() {
		searchProperties[k] = val
	}
	for _, p := range search.GetProperties() {
		searchProperties[p] = search.GetQuery()
	}

	query := framedata.NewSearchQuery(
		search.GetQuery(), searchProperties,
		int(limits.Page),
		int(limits.Count),
	)

	notificationStream, err := nb.notificationRepo.Search(ctx, query)
	if err != nil {
		logger.WithError(err).Error("failed to search notifications")
		return err
	}

	var resultStatus *models.NotificationStatus
	var language *models.Language
	for {

		res, ok := notificationStream.ReadResult(ctx)
		if !ok {
			break
		}

		if res.IsError() {
			return res.Error()
		}

		var responsesList []*notificationv1.Notification

		for _, n := range res.Item() {

			nStatus := &models.NotificationStatus{}
			if n.StatusID != "" {
				resultStatus, err = nb.notificationStatusRepo.GetByID(ctx, n.StatusID)
				if err != nil {
					logger.WithError(err).WithField("status_id", n.StatusID).Error(" could not get status id for")
					return err
				} else {
					nStatus = resultStatus
				}
			}

			language, err = nb.languageRepo.GetByID(ctx, n.LanguageID)
			if err != nil {
				logger.WithError(err).WithField("language_id", n.LanguageID).Error(" could not get language id")
				return err
			}

			result := n.ToApi(nStatus, language, nil)
			responsesList = append(responsesList, result)
		}

		err = stream.Send(&notificationv1.SearchResponse{Data: responsesList})
		if err != nil {
			logger.WithError(err).Warn(" unable to send a result")
		}
	}

	return nil
}

func (nb *notificationBusiness) TemplateSearch(search *notificationv1.TemplateSearchRequest,
	stream notificationv1.NotificationService_TemplateSearchServer) error {

	ctx := stream.Context()
	logger := nb.service.Log(ctx).WithField("request", search)

	queryString := search.GetQuery()

	authClaims := frame.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Debug("handling template search request")

	templateList, err := nb.templateRepo.SearchByName(ctx, queryString, int(search.GetPage()), int(search.GetCount()))
	if err != nil {
		return err
	}

	var language *models.Language
	if search.GetLanguageCode() != "" {
		language, err = nb.languageRepo.GetOrCreateByCode(ctx, search.GetLanguageCode())

		if err != nil {
			return err
		}
	}

	var responseList []*notificationv1.Template
	var templateDataList []*models.TemplateData
	languageMap := map[string]*models.Language{}

	for _, t := range templateList {

		var apiTemplateDataList []*notificationv1.TemplateData

		templateDataList, err = nb.templateDataRepo.GetByTemplateID(ctx, t.GetID())
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

				lang, err = nb.languageRepo.GetByID(ctx, data.LanguageID)
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

	err = stream.Send(&notificationv1.TemplateSearchResponse{Data: responseList})
	if err != nil {
		logger.WithError(err).Warn(" unable to send a result")
		return err
	}

	return nil
}

func (nb *notificationBusiness) TemplateSave(ctx context.Context, req *notificationv1.TemplateSaveRequest) (*notificationv1.Template, error) {
	logger := nb.service.Log(ctx).WithField("request", req)

	authClaims := frame.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Info("handling template request update")

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, req.GetLanguageCode())

	if err != nil {
		logger.WithError(err).Debug("language for template is required")
		return nil, err
	}

	template := &models.Template{
		Name:  req.GetName(),
		Extra: frame.DBPropertiesFromMap(req.GetExtra()),
	}

	err = nb.templateRepo.Save(ctx, template)
	if err != nil {
		return nil, err
	}

	for key, val := range req.GetData() {
		templateData := &models.TemplateData{
			TemplateID: template.GetID(),
			LanguageID: language.GetID(),
			Type:       key,
			Detail:     val,
		}

		err = nb.templateDataRepo.Save(ctx, templateData)
		if err != nil {
			return nil, err
		}
	}

	template, err = nb.templateRepo.GetByID(ctx, template.GetID())
	if err != nil {
		logger.WithError(err).Debug("could not get existing template")
		return nil, err
	}

	languageMap := map[string]*models.Language{}

	var apiTemplateDataList []*notificationv1.TemplateData

	templateDataList, err0 := nb.templateDataRepo.GetByTemplateID(ctx, template.GetID())
	if err0 != nil {
		logger.WithError(err0).Debug("could not get existing template data")
		return nil, err
	}
	for _, data := range templateDataList {

		lang, ok := languageMap[data.LanguageID]
		if !ok {

			lang, err = nb.languageRepo.GetByID(ctx, data.LanguageID)
			if err != nil {
				return nil, err
			}
		}

		apiTemplateDataList = append(apiTemplateDataList, data.ToApi(lang.ToApi()))
	}

	return template.ToApi(apiTemplateDataList), nil
}
