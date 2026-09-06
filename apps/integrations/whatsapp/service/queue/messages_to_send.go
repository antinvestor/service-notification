// Package queue consumes notifications routed to WhatsApp and dispatches them.
package queue

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/client"
	"github.com/antinvestor/service-notification/pkg/events"
	frameEvents "github.com/pitabwire/frame/v2/events"
	"github.com/pitabwire/frame/v2/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// statusEmitter is the slice of the frame events manager the worker needs.
type statusEmitter interface {
	Emit(ctx context.Context, name string, payload any) error
}

var _ statusEmitter = frameEvents.Manager(nil)

type messageToSend struct {
	eventsMan   statusEmitter
	whatsAppCli *client.Client
}

// NewMessageToSend returns the subscriber worker for the WhatsApp send queue.
func NewMessageToSend(eventsMan statusEmitter, whatsAppCli *client.Client) queue.SubscribeWorker {
	return &messageToSend{eventsMan: eventsMan, whatsAppCli: whatsAppCli}
}

// Handle sends one notification. Transient provider failures return an error so the
// queue redelivers; permanent ones are reported as FAILED (with a fallback channel when
// the recipient is unreachable on WhatsApp) and acknowledged.
func (ms *messageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {
	log := util.Log(ctx).WithField("type", "whatsapp.message.send")
	defer log.Release()

	notification := &notificationv1.Notification{}
	if err := proto.Unmarshal(payload, notification); err != nil {
		log.WithError(err).Error("failed to unmarshal notification, dropping")
		return nil
	}

	log = log.WithField("notification_id", notification.GetId())
	log.Debug("queue handler started")

	result, err := ms.whatsAppCli.Send(ctx, headers, notification)
	if err != nil {
		return ms.reportFailure(ctx, log, notification.GetId(), err)
	}

	extra, _ := structpb.NewStruct(map[string]any{
		"wa_id":    result.WaID,
		"template": result.Template,
		"channel":  "whatsapp",
	})
	ms.emitStatus(ctx, log, &commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_ACTIVE,
		Status:     commonv1.STATUS_QUEUED,
		ExternalId: result.MessageID,
		Extras:     extra,
	})

	log.WithFields(map[string]any{"wamid": result.MessageID, "template": result.Template}).Info("message accepted by WhatsApp")
	return nil
}

func (ms *messageToSend) reportFailure(ctx context.Context, log *util.LogEntry, notificationID string, err error) error {
	extras := map[string]any{"error": err.Error(), "channel": "whatsapp"}

	var sendErr *client.SendError
	if errors.As(err, &sendErr) {
		extras["error_code"] = sendErr.Code
		if sendErr.TraceID != "" {
			extras["fbtrace_id"] = sendErr.TraceID
		}

		if sendErr.Retriable() {
			log.WithError(err).Warn("transient WhatsApp failure, message will be retried")
			extra, _ := structpb.NewStruct(extras)
			ms.emitStatus(ctx, log, &commonv1.StatusUpdateRequest{
				Id: notificationID, State: commonv1.STATE_ACTIVE, Status: commonv1.STATUS_UNKNOWN, Extras: extra,
			})
			return err
		}

		if fallback := sendErr.Fallback(); fallback != "" {
			extras["fallback_channel"] = fallback
		}
		if sendErr.Credential() {
			log.WithError(err).Error("WhatsApp credentials rejected, operator attention required")
		}
	}

	log.WithError(err).Error("WhatsApp delivery failed permanently")
	extra, _ := structpb.NewStruct(extras)
	ms.emitStatus(ctx, log, &commonv1.StatusUpdateRequest{
		Id: notificationID, State: commonv1.STATE_INACTIVE, Status: commonv1.STATUS_FAILED, Extras: extra,
	})
	return nil
}

func (ms *messageToSend) emitStatus(ctx context.Context, log *util.LogEntry, req *commonv1.StatusUpdateRequest) {
	if err := ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent, req); err != nil {
		log.WithError(err).Warn("could not emit status update")
	}
}
