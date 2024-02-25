package business

import (
	"context"
	"fmt"
	"github.com/antinvestor/service-notification/config"
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *notificationV1.Notification) (*commonv1.StatusResponse, error)
	QueueIn(ctx context.Context, in *notificationV1.Notification) (*commonv1.StatusResponse, error)
	Status(ctx context.Context, status *commonv1.StatusRequest) (*commonv1.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error)
	Release(ctx context.Context, status *notificationV1.ReleaseRequest) (*commonv1.StatusResponse, error)
	Search(search *commonv1.SearchRequest, stream notificationV1.NotificationService_SearchServer) error
	TemplateSave(ctx context.Context, req *notificationV1.TemplateSaveRequest) (*notificationV1.Template, error)
	TemplateSearch(search *notificationV1.TemplateSearchRequest, stream notificationV1.NotificationService_TemplateSearchServer) error
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

func getLanguageByCode(ctx context.Context, service *frame.Service, languageCode string) (*models.Language, error) {

	if languageCode == "" {
		notificationConfig, ok := service.Config().(*config.NotificationConfig)
		if ok {
			languageCode = notificationConfig.DefaultLanguageCode
		}
	}

	languageRepo := repository.NewLanguageRepository(ctx, service)
	lang, err := languageRepo.GetByCode(languageCode)
	if err != nil {
		if !frame.DBErrorIsRecordNotFound(err) || languageCode == "" {
			return nil, err
		}

		lang = &models.Language{
			BaseModel:   frame.BaseModel{},
			Name:        fmt.Sprintf("Edit - %s", languageCode),
			Code:        languageCode,
			Description: "Auto created partition language",
		}
		lang.GenID(ctx)

		err = languageRepo.Save(lang)
		if err != nil {
			return nil, err

		}
	}

	return lang, nil
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.L().WithField("request", message)
	logger.Info("handling queue out request")

	var releaseDate time.Time
	if message.AutoRelease {
		releaseDate = time.Now()
	}

	language, err := getLanguageByCode(ctx, nb.service, message.GetLanguage())
	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	profileID := message.GetProfileId()

	var contact string
	contactObj := message.GetContact()
	contactData, ok := contactObj.(*notificationV1.Notification_ContactId)
	if ok {
		contact = contactData.ContactId
	} else {
		contactDetail, ok0 := contactObj.(*notificationV1.Notification_Detail)
		if ok0 {
			contact = contactDetail.Detail
		}
	}

	if contact != "" {

		var profile *profileV1.ProfileObject
		profile, err = nb.profileCli.GetProfileByContact(ctx, contact)
		if err != nil {
			logger.WithError(err).Warn("could not obtain contact")

			profile, err = nb.profileCli.CreateProfileByContactAndName(ctx, contact, "")
			if err != nil {
				logger.WithError(err).Warn("could not create contact")
				return nil, err
			}
		}

		if profile.GetId() != "" && profile.GetId() != profileID {
			profileID = profile.GetId()
		}
		for _, pcontact := range profile.GetContacts() {
			if pcontact.GetDetail() == contact {
				contact = pcontact.GetId()
				break
			}
		}

	}

	if err != nil {
		logger.WithError(err).Warn("could not get/match contact")
		return nil, err
	}

	profileType := message.GetProfileType()
	if profileType == "" {
		profileType = "Profile"
	}

	n := models.Notification{
		ParentID:    message.GetParentId(),
		TransientID: message.GetId(),
		ContactID:   contact,
		ProfileID:   profileID,
		ProfileType: profileType,

		Source:          message.GetSource(),
		SourceContactID: message.GetSourceContactId(),

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
		templateRepo := repository.NewTemplateRepository(ctx, nb.service)
		t, err := templateRepo.GetByName(message.GetTemplate())
		if err != nil {
			logger.WithError(err).Warn("could not get template")
			return nil, err
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

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationV1.Notification) (*commonv1.StatusResponse, error) {
	logger := nb.service.L().WithField("request", message)
	logger.Info("handling queue in request")

	releaseDate := time.Now()

	language, err := getLanguageByCode(ctx, nb.service, message.GetLanguage())
	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	profileType := message.GetProfileType()
	if profileType == "" {
		profileType = "Profile"
	}

	n := models.Notification{
		ParentID:    message.GetParentId(),
		TransientID: message.GetId(),
		ProfileType: profileType,
		ProfileID:   message.GetProfileId(),
		ContactID:   message.GetContactId(),

		Source:          message.GetSource(),
		SourceContactID: message.GetSourceContactId(),

		RouteID:    message.GetRouteId(),
		LanguageID: language.GetID(),
		OutBound:   false,

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
	logger := nb.service.L().WithField("request", statusReq)
	logger.Info("handling status check request")

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByID(statusReq.GetId())
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
	logger := nb.service.L().WithField("request", statusReq)
	logger.Debug("handling status update request")

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	n, err := notificationRepo.GetByID(statusReq.GetId())
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

	logger := nb.service.L().WithField("request", releaseReq)
	logger.Debug("handling release request")

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)
	n, err := notificationRepo.GetByID(releaseReq.GetId())
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
	logger := nb.service.L().WithField("request", search)

	logger.Debug("handling search request")

	ctx := stream.Context()
	jwtToken := frame.JwtFromContext(ctx)

	logger.WithField("jwt", jwtToken).Debug("auth jwt supplied")

	var notificationList []*models.Notification
	var err error

	notificationRepo := repository.NewNotificationRepository(ctx, nb.service)

	if search.GetIdQuery() != "" {
		notification, err0 := notificationRepo.GetByID(search.GetIdQuery())
		if err0 != nil {
			return err0
		}

		notificationList = append(notificationList, notification)

	} else {

		notificationList, err = notificationRepo.Search(search.GetQuery())
		if err != nil {
			logger.WithError(err).Error("failed to search notifications")
			return err
		}

	}

	var responsesList []*notificationV1.Notification
	for _, n := range notificationList {
		nStatus := &models.NotificationStatus{}
		if n.StatusID != "" {
			notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
			resultStatus, err := notificationStatusRepo.GetByID(n.StatusID)
			if err != nil {
				logger.WithError(err).WithField("status_id", n.StatusID).Error(" could not get status id for")
				return err
			} else {
				nStatus = resultStatus
			}
		}

		languageRepo := repository.NewLanguageRepository(ctx, nb.service)
		language, err := languageRepo.GetByID(n.LanguageID)
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
	logger := nb.service.L().WithField("request", search)

	logger.Debug("handling template search request")

	ctx := stream.Context()
	queryString := search.GetQuery()

	templateRepository := repository.NewTemplateRepository(ctx, nb.service)
	templateList, err := templateRepository.SearchByName(queryString, int(search.GetPage()), int(search.GetCount()))
	if err != nil {
		return err
	}

	languageRepo := repository.NewLanguageRepository(ctx, nb.service)

	var language *models.Language
	if search.GetLanguageCode() != "" {
		language, err = getLanguageByCode(ctx, nb.service, search.GetLanguageCode())
		if err != nil {
			return err
		}
	}

	templateDataRepository := repository.NewTemplateDataRepository(ctx, nb.service)

	var responseList []*notificationV1.Template
	var templateDataList []*models.TemplateData
	languageMap := map[string]*models.Language{}

	for _, t := range templateList {

		var apiTemplateDataList []*notificationV1.TemplateData

		templateDataList, err = templateDataRepository.GetByTemplateID(t.GetID())
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

				lang, err = languageRepo.GetByID(data.LanguageID)
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
	logger := nb.service.L().WithField("request", req)
	logger.Info("handling template request update")

	language, err := getLanguageByCode(ctx, nb.service, req.GetLanguageCode())
	if err != nil {
		logger.WithError(err).Debug("language for template is required")
		return nil, err
	}

	template := &models.Template{
		Name:  req.GetName(),
		Extra: frame.DBPropertiesFromMap(req.GetExtra()),
	}

	templateRepository := repository.NewTemplateRepository(ctx, nb.service)
	err = templateRepository.Save(template)
	if err != nil {
		return nil, err
	}

	templateDataRepository := repository.NewTemplateDataRepository(ctx, nb.service)

	for key, val := range req.GetData() {
		templateData := &models.TemplateData{
			TemplateID: template.GetID(),
			LanguageID: language.GetID(),
			Type:       key,
			Detail:     val,
		}

		err = templateDataRepository.Save(templateData)
		if err != nil {
			return nil, err
		}
	}

	template, err = templateRepository.GetByID(template.GetID())
	if err != nil {
		logger.WithError(err).Debug("could not get existing template")
		return nil, err
	}

	languageRepo := repository.NewLanguageRepository(ctx, nb.service)

	languageMap := map[string]*models.Language{}

	var apiTemplateDataList []*notificationV1.TemplateData

	templateDataList, err0 := templateDataRepository.GetByTemplateID(template.GetID())
	if err0 != nil {
		logger.WithError(err0).Debug("could not get existing template data")
		return nil, err
	}
	for _, data := range templateDataList {

		lang, ok := languageMap[data.LanguageID]
		if !ok {

			lang, err = languageRepo.GetByID(data.LanguageID)
			if err != nil {
				return nil, err
			}
		}

		apiTemplateDataList = append(apiTemplateDataList, data.ToApi(lang.ToApi()))
	}

	return template.ToApi(apiTemplateDataList), nil
}
