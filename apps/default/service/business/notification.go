package business

import (
	"context"
	"fmt"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/partition/connectrpc/go/partition/v1/partitionv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/data"
	fevents "github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/workerpool"
	"github.com/pitabwire/util"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *notificationv1.Notification) (*commonv1.StatusResponse, error)
	QueueIn(ctx context.Context, in *notificationv1.Notification) (*commonv1.StatusResponse, error)
	Status(ctx context.Context, status *commonv1.StatusRequest) (*commonv1.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error)
	Release(ctx context.Context, req *notificationv1.ReleaseRequest) (workerpool.JobResultPipe[*notificationv1.ReleaseResponse], error)
	Search(ctx context.Context, search *commonv1.SearchRequest, consumer func(ctx context.Context, batch []*notificationv1.Notification) error) error
	TemplateSave(ctx context.Context, req *notificationv1.TemplateSaveRequest) (*notificationv1.Template, error)
	TemplateSearch(ctx context.Context, search *notificationv1.TemplateSearchRequest, consumer func(ctx context.Context, batch []*notificationv1.Template) error) error
}

func NewNotificationBusiness(_ context.Context,
	workMan workerpool.Manager, eventsMan fevents.Manager,
	profileCli profilev1connect.ProfileServiceClient, partitionCli partitionv1connect.PartitionServiceClient,
	notificationRepo repository.NotificationRepository,
	notificationStatusRepo repository.NotificationStatusRepository,
	languageRepo repository.LanguageRepository,
	templateRepo repository.TemplateRepository,
	templateDataRepo repository.TemplateDataRepository,
	routeRepo repository.RouteRepository,
) NotificationBusiness {
	return &notificationBusiness{
		workMan:                workMan,
		eventsMan:              eventsMan,
		profileCli:             profileCli,
		partitionCli:           partitionCli,
		notificationRepo:       notificationRepo,
		notificationStatusRepo: notificationStatusRepo,
		languageRepo:           languageRepo,
		templateRepo:           templateRepo,
		templateDataRepo:       templateDataRepo,
		routeRepo:              routeRepo,
	}
}

type notificationBusiness struct {
	eventsMan              fevents.Manager
	workMan                workerpool.Manager
	profileCli             profilev1connect.ProfileServiceClient
	partitionCli           partitionv1connect.PartitionServiceClient
	notificationRepo       repository.NotificationRepository
	notificationStatusRepo repository.NotificationStatusRepository
	languageRepo           repository.LanguageRepository
	templateRepo           repository.TemplateRepository
	templateDataRepo       repository.TemplateDataRepository
	routeRepo              repository.RouteRepository
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationv1.Notification) (*commonv1.StatusResponse, error) {
	logger := util.Log(ctx).WithField("request", message)

	authClaim := security.ClaimsFromContext(ctx)

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

	templateID := ""
	if message.GetTemplate() != "" {
		t, err0 := nb.templateRepo.GetByName(ctx, message.GetTemplate())
		if err0 != nil {
			logger.WithError(err0).Warn("could not get template")
			return nil, err0
		}

		templateID = t.GetID()
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

		TemplateID: templateID,

		Payload: message.Payload.AsMap(),
		Message: message.GetData(),

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
		Status:         int32(commonv1.STATUS_QUEUED.Number()),
	}

	nStatus.GenID(ctx)

	// Queue out message for further processing
	err = nb.eventsMan.Emit(ctx, events.NotificationSaveEvent, n)
	if err != nil {
		logger.WithError(err).Warn("could not emit event save")
		return nil, err
	}

	// Queue out notification status for further processing
	err = nb.eventsMan.Emit(ctx, events.NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToAPI(), nil
}

func (nb *notificationBusiness) QueueIn(ctx context.Context, message *notificationv1.Notification) (*commonv1.StatusResponse, error) {
	logger := util.Log(ctx).WithField("request", message)

	authClaim := security.ClaimsFromContext(ctx)
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

		Payload:          message.GetPayload().AsMap(),
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
	err = nb.eventsMan.Emit(ctx, events.NotificationSaveEvent, n)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification")
		return nil, err
	}

	// Queue out notification status for further processing
	err = nb.eventsMan.Emit(ctx, events.NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not emit notification status")
		return nil, err
	}

	return nStatus.ToAPI(), nil
}

func (nb *notificationBusiness) Status(ctx context.Context, statusReq *commonv1.StatusRequest) (*commonv1.StatusResponse, error) {
	logger := util.Log(ctx).WithField("request", statusReq)
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
	return nStatus.ToAPI(), nil
}

func (nb *notificationBusiness) StatusUpdate(ctx context.Context, statusReq *commonv1.StatusUpdateRequest) (*commonv1.StatusResponse, error) {
	logger := util.Log(ctx).WithField("request", statusReq)
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
		Extra:          statusReq.GetExtras().AsMap(),
	}

	nStatus.GenID(ctx)

	// Queue out notification status for further processing
	err = nb.eventsMan.Emit(ctx, events.NotificationStatusSaveEvent, nStatus)
	if err != nil {
		logger.WithError(err).Warn("could not save status")
		return nil, err
	}

	return nStatus.ToAPI(), nil
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationv1.ReleaseRequest) (workerpool.JobResultPipe[*notificationv1.ReleaseResponse], error) {

	job := workerpool.NewJob(func(ctx context.Context, resultPipe workerpool.JobResultPipe[*notificationv1.ReleaseResponse]) error {

		logger := util.Log(ctx).WithField("request", releaseReq)
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

			err = nb.eventsMan.Emit(ctx, events.NotificationSaveEvent, n)
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
			err = nb.eventsMan.Emit(ctx, events.NotificationStatusSaveEvent, nStatus)
			if err != nil {
				logger.WithError(err).Warn("could not emit notification status")
				return err
			}

			statusesToRelease = append(statusesToRelease, nStatus.ToAPI())
		}

		if len(statusesToRelease) > 0 {
			err = resultPipe.WriteResult(ctx, &notificationv1.ReleaseResponse{Data: statusesToRelease})
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
				statusesToRelease = append(statusesToRelease, nStatus.ToAPI())
			}
		}

		if len(statusesToRelease) > 0 {
			err = resultPipe.WriteResult(ctx, &notificationv1.ReleaseResponse{Data: statusesToRelease})
			if err != nil {
				return err
			}
		}

		return nil
	})

	err := workerpool.SubmitJob(ctx, nb.workMan, job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (nb *notificationBusiness) convertNotificationsToAPI(
	ctx context.Context,
	notificationList []*models.Notification,
) ([]*notificationv1.Notification, error) {
	var responsesList []*notificationv1.Notification

	var statusIDList []string
	languageIDMap := map[string]struct{}{}

	for _, p := range notificationList {
		statusIDList = append(statusIDList, p.StatusID)

		languageIDMap[p.LanguageID] = struct{}{}
	}

	languageIDList := make([]string, 0, len(languageIDMap))
	for key := range languageIDMap {
		languageIDList = append(languageIDList, key)
	}

	languageList, err := nb.languageRepo.GetByIDList(ctx, languageIDList...)
	if err != nil {
		return nil, err
	}

	languageMap := make(map[string]*models.Language)
	for _, language := range languageList {
		languageMap[language.ID] = language
	}

	statusList, err := nb.notificationStatusRepo.GetByIDList(ctx, statusIDList...)
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]*models.NotificationStatus)
	for _, status := range statusList {
		statusMap[status.ID] = status
	}

	for _, not := range notificationList {
		status := statusMap[not.ID]
		language := languageMap[not.LanguageID]

		// Convert the payment model to the API response format
		result := not.ToAPI(status, language, nil)
		responsesList = append(responsesList, result)
	}

	return responsesList, nil
}

func (nb *notificationBusiness) Search(ctx context.Context, searchQuery *commonv1.SearchRequest, consumer func(ctx context.Context, batch []*notificationv1.Notification) error) error {

	logger := util.Log(ctx).WithField("request", searchQuery)

	logger.Debug("handling search request")

	limits := searchQuery.GetLimits()

	searchOpts := []data.SearchOption{
		data.WithSearchLimit(int(limits.GetCount())),
		data.WithSearchOffset(int(limits.GetPage())),
	}

	andQueryVal := map[string]any{}

	for k, v := range searchQuery.GetExtras().AsMap() {
		andQueryVal[fmt.Sprintf("%s = ?", k)] = v
	}

	if searchQuery.GetIdQuery() != "" {
		andQueryVal["id = ?"] = searchQuery.GetIdQuery()
	}

	if len(andQueryVal) > 0 {
		searchOpts = append(
			searchOpts,
			data.WithSearchFiltersAndByValue(andQueryVal))
	}

	if searchQuery.GetQuery() != "" {
		searchOpts = append(
			searchOpts,
			data.WithSearchFiltersOrByValue(
				map[string]any{"searchable @@ websearch_to_tsquery( 'english', ?) ": searchQuery.GetQuery()},
			),
		)

		for _, filter := range searchQuery.GetProperties() {
			searchOpts = append(
				searchOpts,
				data.WithSearchFiltersOrByValue(map[string]any{fmt.Sprintf(" %s = ?", filter): searchQuery.GetQuery()}),
			)
		}
	}

	query := data.NewSearchQuery(searchOpts...)
	results, err := nb.notificationRepo.Search(ctx, query)
	if err != nil {
		logger.WithError(err).Error("failed to search notifications")
		return err
	}

	return workerpool.ConsumeResultStream(ctx, results, func(res []*models.Notification) error {
		finalRes, convErr := nb.convertNotificationsToAPI(ctx, res)
		if convErr != nil {
			return convErr
		}

		consumeErr := consumer(ctx, finalRes)
		if consumeErr != nil {
			return consumeErr
		}
		return nil
	})

}

func (nb *notificationBusiness) convertTemplatesToAPI(ctx context.Context, language *models.Language, templateList []*models.Template) ([]*notificationv1.Template, error) {
	var responsesList []*notificationv1.Template

	var templateIDList []string

	for _, p := range templateList {
		templateIDList = append(templateIDList, p.GetID())
	}

	var err error
	var templateDataList []*models.TemplateData

	if language != nil {

		templateDataList, err = nb.templateDataRepo.GetByTemplateIDAndLanguage(ctx, language.GetID(), templateIDList...)
		if err != nil {
			return nil, err
		}
	} else {
		templateDataList, err = nb.templateDataRepo.GetByTemplateID(ctx, templateIDList...)
		if err != nil {
			return nil, err
		}
	}

	languageIDMap := map[string]struct{}{}

	for _, tData := range templateDataList {
		languageIDMap[tData.LanguageID] = struct{}{}
	}

	languageIDList := make([]string, 0, len(languageIDMap))
	for key := range languageIDMap {
		languageIDList = append(languageIDList, key)
	}

	languageList, err := nb.languageRepo.GetByIDList(ctx, languageIDList...)
	if err != nil {
		return nil, err
	}

	languageMap := make(map[string]*models.Language)
	for _, l := range languageList {
		languageMap[l.ID] = l
	}

	apiTDataMap := map[string][]*notificationv1.TemplateData{}

	for _, tmplData := range templateDataList {

		lang := languageMap[tmplData.LanguageID]
		apiTDataMap[tmplData.TemplateID] = append(apiTDataMap[tmplData.TemplateID], tmplData.ToApi(lang.ToApi()))
	}

	for _, tmpl := range templateList {
		tDataList := apiTDataMap[tmpl.ID]
		result := tmpl.ToApi(tDataList)
		responsesList = append(responsesList, result)
	}

	return responsesList, nil
}

func (nb *notificationBusiness) TemplateSearch(ctx context.Context, searchQuery *notificationv1.TemplateSearchRequest, consumer func(ctx context.Context, batch []*notificationv1.Template) error) error {

	logger := util.Log(ctx).WithField("request", searchQuery)

	authClaims := security.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Debug("handling template searchQuery request")

	searchOpts := []data.SearchOption{
		data.WithSearchLimit(int(searchQuery.GetCount())),
		data.WithSearchOffset(int(searchQuery.GetPage())),
	}

	if searchQuery.GetQuery() != "" {
		searchOpts = append(
			searchOpts,
			data.WithSearchFiltersOrByValue(
				map[string]any{"searchable @@ websearch_to_tsquery( 'english', ?) ": searchQuery.GetQuery()},
			),
		)
	}

	var err error
	var language *models.Language
	if searchQuery.GetLanguageCode() != "" {
		language, err = nb.languageRepo.GetOrCreateByCode(ctx, searchQuery.GetLanguageCode())

		if err != nil {
			return err
		}
	}

	query := data.NewSearchQuery(searchOpts...)

	templateList, err := nb.templateRepo.Search(ctx, query)
	if err != nil {
		return err
	}

	for {
		res, ok := templateList.ReadResult(ctx)
		if !ok {
			return nil
		}

		if res.IsError() {
			return res.Error()
		}

		finalRes, convErr := nb.convertTemplatesToAPI(ctx, language, res.Item())
		if convErr != nil {
			return convErr
		}

		writeErr := consumer(ctx, finalRes)
		if writeErr != nil {
			return writeErr
		}
	}
}

func (nb *notificationBusiness) TemplateSave(ctx context.Context, req *notificationv1.TemplateSaveRequest) (*notificationv1.Template, error) {
	logger := util.Log(ctx).WithField("request", req)

	authClaims := security.ClaimsFromContext(ctx)
	logger.WithField("claims", authClaims).Info("handling template request update")

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, req.GetLanguageCode())

	if err != nil {
		logger.WithError(err).Debug("language for template is required")
		return nil, err
	}

	template := &models.Template{
		Name:  req.GetName(),
		Extra: req.GetExtra().AsMap(),
	}

	err = nb.templateRepo.Create(ctx, template)
	if err != nil {
		return nil, err
	}

	for key, val := range req.GetData().AsMap() {
		templateData := &models.TemplateData{
			TemplateID: template.GetID(),
			LanguageID: language.GetID(),
			Type:       key,
			Detail:     val.(string),
		}

		err = nb.templateDataRepo.Create(ctx, templateData)
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
		logger.WithError(err0).Debug("could not get existing template tData")
		return nil, err
	}
	for _, tData := range templateDataList {

		lang, ok := languageMap[tData.LanguageID]
		if !ok {

			lang, err = nb.languageRepo.GetByID(ctx, tData.LanguageID)
			if err != nil {
				return nil, err
			}
		}

		apiTemplateDataList = append(apiTemplateDataList, tData.ToApi(lang.ToApi()))
	}

	return template.ToApi(apiTemplateDataList), nil
}
