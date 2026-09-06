package business

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/v2/data"
	fevents "github.com/pitabwire/frame/v2/events"
	"github.com/pitabwire/frame/v2/workerpool"
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
	profileCli profilev1connect.ProfileServiceClient, tenancyCli tenancyv1connect.TenancyServiceClient,
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
		tenancyCli:             tenancyCli,
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
	tenancyCli             tenancyv1connect.TenancyServiceClient
	notificationRepo       repository.NotificationRepository
	notificationStatusRepo repository.NotificationStatusRepository
	languageRepo           repository.LanguageRepository
	templateRepo           repository.TemplateRepository
	templateDataRepo       repository.TemplateDataRepository
	routeRepo              repository.RouteRepository
}

func (nb *notificationBusiness) QueueOut(ctx context.Context, message *notificationv1.Notification) (*commonv1.StatusResponse, error) {
	logger := util.Log(ctx)

	logger.Debug("handling queue out request")

	n := models.NotificationFromAPI(ctx, message)
	n.OutBound = true
	if message.AutoRelease {
		releaseDate := time.Now()
		n.ReleasedAt = &releaseDate
	}

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	n.LanguageID = language.GetID()

	templateID := ""
	if message.GetTemplate() != "" {
		t, err0 := nb.templateRepo.GetByName(ctx, message.GetTemplate())
		if err0 != nil {
			logger.WithError(err0).Warn("could not get template")
			return nil, err0
		}

		templateID = t.GetID()
	}

	n.TemplateID = templateID

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
	logger := util.Log(ctx)

	logger.Debug("handling queue in request")

	n := models.NotificationFromAPI(ctx, message)
	n.OutBound = false
	releaseDate := time.Now()
	n.ReleasedAt = &releaseDate

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, message.GetLanguage())

	if err != nil {
		logger.WithError(err).Warn("could not get language")
		return nil, err
	}

	n.LanguageID = language.GetID()

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
	logger := util.Log(ctx).WithField("notification_id", statusReq.GetId())
	logger.Debug("handling status check request")

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
	logger := util.Log(ctx).WithFields(map[string]any{
		"notification_id": statusReq.GetId(),
		"external_id":     statusReq.GetExternalId(),
	})
	logger.Debug("handling status update request")

	n, err := nb.resolveNotification(ctx, statusReq.GetId(), statusReq.GetExternalId())
	if err != nil {
		logger.WithError(err).Warn("could not resolve notification for status update")
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

// resolveNotification loads a notification by id, or by the provider-assigned external id
// when no id is given (delivery reports only know the provider's message id).
func (nb *notificationBusiness) resolveNotification(ctx context.Context, id, externalID string) (*models.Notification, error) {
	if id != "" {
		return nb.notificationRepo.GetByID(ctx, id)
	}
	if externalID != "" {
		return nb.notificationRepo.GetByExternalID(ctx, externalID)
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("notification id or external id is required"))
}

func (nb *notificationBusiness) Release(ctx context.Context, releaseReq *notificationv1.ReleaseRequest) (workerpool.JobResultPipe[*notificationv1.ReleaseResponse], error) {

	job := workerpool.NewJob(func(ctx context.Context, resultPipe workerpool.JobResultPipe[*notificationv1.ReleaseResponse]) error {

		logger := util.Log(ctx)
		logger.Debug("handling release request")

		notificationList, err := nb.notificationRepo.GetByIDList(ctx, releaseReq.GetId()...)
		if err != nil {
			logger.WithError(err).Warn("could not fetch by id")
			return err
		}

		var releasedStatusIDs []string
		var alreadyReleased, notificationsToUpdate []*models.Notification

		releaseDate := time.Now()

		for _, n := range notificationList {

			if n.IsReleased() {
				alreadyReleased = append(alreadyReleased, n)
				if n.StatusID != "" {
					releasedStatusIDs = append(releasedStatusIDs, n.StatusID)
				}
			} else {
				n.ReleasedAt = &releaseDate
				notificationsToUpdate = append(notificationsToUpdate, n)
			}

		}

		var statusesToRelease []*commonv1.StatusResponse
		for _, n := range notificationsToUpdate {

			// The row already exists, so re-emitting notification.save would be dropped as a
			// duplicate. Persist the release directly and hand off to routing.
			_, err = nb.notificationRepo.Update(ctx, n, "released_at")
			if err != nil {
				logger.WithError(err).WithField("notification_id", n.GetID()).Warn("could not persist release")
				return err
			}

			err = nb.eventsMan.Emit(ctx, events.NotificationOutRouteEvent, n.GetID())
			if err != nil {
				logger.WithError(err).WithField("notification_id", n.GetID()).Warn("could not emit notification out route")
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
		if len(releasedStatusIDs) > 0 {
			notificationStatusList, listErr := nb.notificationStatusRepo.GetByIDList(ctx, releasedStatusIDs...)
			if listErr != nil {
				logger.WithError(listErr).Warn("could not get notification status")
				return listErr
			}

			for _, nStatus := range notificationStatusList {
				statusesToRelease = append(statusesToRelease, nStatus.ToAPI())
			}
		}

		// Already-released notifications whose status row has not landed yet still get a
		// response, so callers see one entry per requested id.
		for _, n := range alreadyReleased {
			if n.StatusID == "" {
				statusesToRelease = append(statusesToRelease, &commonv1.StatusResponse{
					Id: n.GetID(), State: commonv1.STATE(n.State), Status: commonv1.STATUS_QUEUED, ExternalId: n.ExternalID,
				})
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
		if p.StatusID != "" {
			statusIDList = append(statusIDList, p.StatusID)
		}
		if p.LanguageID != "" {
			languageIDMap[p.LanguageID] = struct{}{}
		}
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
		status := statusMap[not.StatusID]
		language := languageMap[not.LanguageID]

		// Convert the payment model to the API response format
		result := not.ToAPI(status, language, nil)
		responsesList = append(responsesList, result)
	}

	return responsesList, nil
}

func (nb *notificationBusiness) Search(ctx context.Context, searchQuery *commonv1.SearchRequest, consumer func(ctx context.Context, batch []*notificationv1.Notification) error) error {

	logger := util.Log(ctx)

	logger.Debug("handling search request")

	// Extract pagination from cursor
	var searchOpts []data.SearchOption

	cursor := searchQuery.GetCursor()

	if cursor != nil {

		offset, offsetErr := strconv.Atoi(cursor.GetPage())
		if offsetErr != nil {
			offset = 0
		}

		searchOpts = append(searchOpts, data.WithSearchOffset(offset), data.WithSearchLimit(int(cursor.GetLimit())))
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
		logger.WithError(err).Warn("failed to search notifications")
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

	logger := util.Log(ctx)

	logger.Debug("handling template search request")

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

// templateSubjectKey is the reserved key in TemplateSaveRequest.data carrying the
// message subject (used by email integrations). It is not a channel type.
const templateSubjectKey = "subject"

// templateDataTypeMaxLen mirrors the varchar(10) width of template_data.type.
const templateDataTypeMaxLen = 10

// parseTemplateSaveData splits the request data map into channel bodies and the optional subject,
// validating that every value is a string.
func parseTemplateSaveData(input map[string]any) (channels map[string]string, subject string, err error) {
	channels = make(map[string]string, len(input))
	for key, val := range input {
		str, ok := val.(string)
		if !ok {
			return nil, "", connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("template data value for %q must be a string, got %T", key, val))
		}

		if key == templateSubjectKey {
			subject = str
			continue
		}

		if key == "" || len(key) > templateDataTypeMaxLen {
			return nil, "", connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("template data key %q must be a channel type of 1-%d characters", key, templateDataTypeMaxLen))
		}

		channels[key] = str
	}
	return channels, subject, nil
}

// upsertTemplate returns the template with the given name, creating it if absent. When it already
// exists, extra is merged over the stored extra map. The returned bool is true if the row was created.
func (nb *notificationBusiness) upsertTemplate(ctx context.Context, name string, extra map[string]any) (*models.Template, bool, error) {
	template, err := nb.templateRepo.GetByName(ctx, name)
	if err != nil {
		if !data.ErrorIsNoRows(err) {
			return nil, false, err
		}

		template = &models.Template{Name: name, Extra: extra}
		if err = nb.templateRepo.Create(ctx, template); err != nil {
			return nil, false, err
		}
		return template, true, nil
	}

	if len(extra) == 0 {
		return template, false, nil
	}

	if template.Extra == nil {
		template.Extra = data.JSONMap{}
	}
	for k, v := range extra {
		template.Extra[k] = v
	}

	rows, err := nb.templateRepo.Update(ctx, template, "extra")
	if err != nil {
		return nil, false, err
	}
	if rows == 0 {
		return nil, false, connect.NewError(connect.CodeAborted,
			fmt.Errorf("template %s was modified concurrently, retry", name))
	}
	return template, false, nil
}

// upsertTemplateData writes the body (and subject) for one (template, language, channel type).
func (nb *notificationBusiness) upsertTemplateData(ctx context.Context, templateID, languageID, dataType, detail, subject string) error {
	templateData, err := nb.templateDataRepo.GetByTemplateLanguageAndType(ctx, templateID, languageID, dataType)
	if err != nil {
		if !data.ErrorIsNoRows(err) {
			return err
		}

		return nb.templateDataRepo.Create(ctx, &models.TemplateData{
			TemplateID: templateID,
			LanguageID: languageID,
			Type:       dataType,
			Detail:     detail,
			Subject:    subject,
		})
	}

	templateData.Detail = detail
	templateData.Subject = subject

	rows, err := nb.templateDataRepo.Update(ctx, templateData, "detail", "subject")
	if err != nil {
		return err
	}
	if rows == 0 {
		return connect.NewError(connect.CodeAborted,
			fmt.Errorf("template data %s/%s was modified concurrently, retry", templateID, dataType))
	}
	return nil
}

// TemplateSave registers a template by name. It is idempotent: repeated calls with the same name
// update the existing template's extra map and the per-channel bodies for the given language instead
// of creating duplicates, so consumer services can call it on every deploy.
func (nb *notificationBusiness) TemplateSave(ctx context.Context, req *notificationv1.TemplateSaveRequest) (*notificationv1.Template, error) {
	logger := util.Log(ctx).WithField("template_name", req.GetName())

	logger.Debug("handling template save request")

	if req.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("template name is required"))
	}

	channels, subject, err := parseTemplateSaveData(req.GetData().AsMap())
	if err != nil {
		return nil, err
	}

	language, err := nb.languageRepo.GetOrCreateByCode(ctx, req.GetLanguageCode())
	if err != nil {
		logger.WithError(err).Debug("language for template is required")
		return nil, err
	}

	template, created, err := nb.upsertTemplate(ctx, req.GetName(), req.GetExtra().AsMap())
	if err != nil {
		return nil, err
	}

	for dataType, detail := range channels {
		err = nb.upsertTemplateData(ctx, template.GetID(), language.GetID(), dataType, detail, subject)
		if err != nil {
			return nil, err
		}
	}

	logger.WithField("template_id", template.GetID()).
		WithField("language_code", language.Code).
		WithField("created", created).
		WithField("channels", len(channels)).
		Info("template saved")

	template, err = nb.templateRepo.GetByID(ctx, template.GetID())
	if err != nil {
		logger.WithError(err).Debug("could not get existing template")
		return nil, err
	}

	languageMap := map[string]*models.Language{}

	var apiTemplateDataList []*notificationv1.TemplateData

	templateDataList, err := nb.templateDataRepo.GetByTemplateID(ctx, template.GetID())
	if err != nil {
		logger.WithError(err).Debug("could not get existing template tData")
		return nil, err
	}
	for _, tData := range templateDataList {

		lang, ok := languageMap[tData.LanguageID]
		if !ok {
			lang, err = nb.languageRepo.GetByID(ctx, tData.LanguageID)
			if err != nil {
				return nil, err
			}
			languageMap[tData.LanguageID] = lang
		}

		apiTemplateDataList = append(apiTemplateDataList, tData.ToApi(lang.ToApi()))
	}

	return template.ToApi(apiTemplateDataList), nil
}
