package queues

import (
	"context"
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/antinvestor/service-notification/internal/events"
	frameEvents "github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type messageToSend struct {
	eventsMan       frameEvents.Manager
	profileCli      profilev1connect.ProfileServiceClient
	notificationCli notificationv1connect.NotificationServiceClient
	emailSMTPCli    *client.Client
}

func NewMessageToSend(
	eventsMan frameEvents.Manager,
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	emailSMTPCli *client.Client,
) queue.SubscribeWorker {
	return &messageToSend{
		eventsMan:       eventsMan,
		profileCli:      profileCli,
		notificationCli: notificationCli,
		emailSMTPCli:    emailSMTPCli,
	}
}

func (ms *messageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := util.Log(ctx)

	notification := &notificationv1.Notification{}

	err := proto.Unmarshal(payload, notification)
	if err != nil {
		log.WithError(err).WithField("payload", payload).Error("Failed to unmarshal notification")
		return nil
	}

	log = log.WithField("notification_id", notification.GetId())
	log.WithFields(map[string]any{
		"recipient": notification.GetRecipient().GetProfileId(),
		"sender":    notification.GetSource().GetProfileId(),
		"subject":   notification.GetData()}).
		Debug("Sending Email SMTP message")

	err = ms.emailSMTPCli.Send(ctx, headers, notification)
	if err != nil {
		log.WithError(err).Error("Email SMTP server responded with error")

		extraData := map[string]any{
			"error": err.Error(),
		}
		extra, _ := structpb.NewStruct(extraData)

		var appErr *apperrors.Error
		ok := errors.As(err, &appErr)
		if !ok || appErr.IsRetriable() {

			err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent,
				&commonv1.StatusUpdateRequest{
					Id:         notification.GetId(),
					State:      commonv1.STATE_ACTIVE,
					Status:     commonv1.STATUS_UNKNOWN,
					ExternalId: "",
					Extras:     extra,
				})
			if err != nil {
				log.WithError(err).Warn("could not update status on notification service")
				return nil
			}

			return nil
		}

		extraData["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extraData)

		err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent,
			&commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_INACTIVE,
				Status:     commonv1.STATUS_FAILED,
				ExternalId: "",
				Extras:     extra,
			})
		if err != nil {
			log.WithError(err).Warn("could not update status on notification service")
			return nil
		}
		return nil
	}

	err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent, &commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_INACTIVE,
		Status:     commonv1.STATUS_SUCCESSFUL,
		ExternalId: "",
	})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
		return nil
	}

	log.Debug("Email SMTP message sent successfully")
	return nil
}
