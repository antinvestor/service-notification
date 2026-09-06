package events

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/v2/data"
	"github.com/pitabwire/frame/v2/events"
	"github.com/pitabwire/util"
)

// NotificationStatusSaveEvent is the event name for saving notification status records
const NotificationStatusSaveEvent = "notificationStatus.save"

type NotificationStatusSave struct {
	eventMan               events.Manager
	NotificationRepo       repository.NotificationRepository
	notificationStatusRepo repository.NotificationStatusRepository
}

// NewNotificationStatusSave creates a new NotificationStatusSave event handler
func NewNotificationStatusSave(_ context.Context, eventMan events.Manager, notificationRepo repository.NotificationRepository, notificationStatusRepo repository.NotificationStatusRepository) *NotificationStatusSave {

	return &NotificationStatusSave{
		eventMan:               eventMan,
		NotificationRepo:       notificationRepo,
		notificationStatusRepo: notificationStatusRepo,
	}
}

func (e *NotificationStatusSave) Name() string {
	return NotificationStatusSaveEvent
}

func (e *NotificationStatusSave) PayloadType() any {
	return &models.NotificationStatus{}
}

func (e *NotificationStatusSave) Validate(_ context.Context, payload any) error {
	notificationStatus, ok := payload.(*models.NotificationStatus)
	if !ok {
		return errors.New(" payload is not of type models.NotificationStatus")
	}

	if notificationStatus.GetID() == "" {
		return errors.New(" notificationStatus Id should already have been set ")
	}

	return nil
}

func (e *NotificationStatusSave) Execute(ctx context.Context, payload any) error {
	nStatus := payload.(*models.NotificationStatus)

	logger := util.Log(ctx).WithFields(map[string]any{"type": e.Name(), "notification_id": nStatus.NotificationID})
	defer logger.Release()
	logger.Debug("event handler started")

	isDuplicate := false
	err := e.notificationStatusRepo.Create(ctx, nStatus)
	if err != nil {
		if data.ErrorIsDuplicateKey(err) {
			isDuplicate = true
			logger.Debug("notification status already exists, skipping duplicate")
		} else {
			logger.WithError(err).Error("could not save notification status to db")
			return err
		}
	}

	n, err := e.NotificationRepo.GetByID(ctx, nStatus.NotificationID)
	if err != nil {
		logger.WithError(err).Error("could not get notification from db")
		return err
	}

	n.StatusID = nStatus.ID
	n.State = nStatus.State
	if n.TransientID == "" {
		n.TransientID = nStatus.TransientID
	}
	if nStatus.ExternalID != "" {
		n.ExternalID = nStatus.ExternalID
	}

	_, err = e.NotificationRepo.Update(ctx, n, "status_id", "state", "transient_id", "external_id")
	if err != nil {
		logger.WithError(err).Error("could not save notification update to db")
		return err
	}

	if !isDuplicate {
		recordStatusMetrics(ctx, n, nStatus)
	}

	// Runs on redelivery too: if the first attempt persisted the status but failed to emit
	// the fallback, the retry must still schedule it. The child's deterministic id keeps
	// this idempotent.
	if err = e.scheduleFallback(ctx, logger, n, nStatus); err != nil {
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}

// scheduleFallback retries a failed outbound notification once on the channel the
// integration nominated in the status extra (for example WhatsApp -> SMS when the
// recipient has no WhatsApp account). Children never fall back again.
func (e *NotificationStatusSave) scheduleFallback(ctx context.Context, logger *util.LogEntry, n *models.Notification, nStatus *models.NotificationStatus) error {
	if commonv1.STATUS(nStatus.Status) != commonv1.STATUS_FAILED || !n.OutBound || n.ParentID != "" {
		return nil
	}

	channel := nStatus.Extra.GetString(models.StatusExtraFallbackChannel)
	if channel == "" || channel == n.NotificationType {
		return nil
	}

	child := models.NewFallbackNotification(ctx, n, channel)

	logger = logger.WithFields(map[string]any{
		"fallback_channel":         channel,
		"fallback_notification_id": child.GetID(),
	})

	err := e.eventMan.Emit(ctx, NotificationSaveEvent, child)
	if err != nil {
		logger.WithError(err).Error("could not emit fallback notification save")
		return err
	}

	nStatus.Extra[models.StatusExtraFallbackNotificationID] = child.GetID()
	if _, err = e.notificationStatusRepo.Update(ctx, nStatus, "extra"); err != nil {
		logger.WithError(err).Warn("could not record fallback notification id on parent status")
	}

	logger.Info("scheduled delivery retry on fallback channel")
	return nil
}
