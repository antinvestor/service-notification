package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/antinvestor/service-notification/internal/events"
	frameEvents "github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type messageToSend struct {
	eventsMan         frameEvents.Manager
	profileCli        profilev1connect.ProfileServiceClient
	notificationCli   notificationv1connect.NotificationServiceClient
	africasTalkingCli *client.Client
}

func NewMessageToSend(
	eventsMan frameEvents.Manager,
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	africasTalkingCli *client.Client,
) queue.SubscribeWorker {
	return &messageToSend{
		eventsMan:         eventsMan,
		profileCli:        profileCli,
		notificationCli:   notificationCli,
		africasTalkingCli: africasTalkingCli,
	}
}

func (ms *messageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := util.Log(ctx).WithField("type", "africastalking.message.send")
	defer log.Release()
	log.Debug("queue handler started")

	notification := notificationv1.Notification{}

	err := proto.Unmarshal(payload, &notification)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal notification")
		return nil
	}

	log = log.WithField("notification_id", notification.GetId())
	log.WithFields(map[string]any{
		"recipient":      notification.GetRecipient().GetProfileId(),
		"sender":         notification.GetSource().GetProfileId(),
		"message_length": len(notification.GetData()),
	}).Debug("processing Africa's Talking SMS message")

	resp, err := ms.africasTalkingCli.Send(ctx, headers, &notification)
	if err != nil {
		log.WithError(err).Error("Africa's Talking API responded with error")

		extrasMap := map[string]any{
			"error": err.Error(),
		}
		extra, _ := structpb.NewStruct(extrasMap)

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
				log.WithError(err).
					Warn("could not update status on notification service")
				return nil
			}

			return nil
		}

		extrasMap["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extrasMap)
		err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent,
			&commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_INACTIVE,
				Status:     commonv1.STATUS_FAILED,
				ExternalId: "",
				Extras:     extra,
			})
		if err != nil {
			log.WithError(err).
				Warn("could not update status on notification service")
			return nil
		}
		return nil
	}

	rs := resp.SMSMessageData.Recipients[0]

	extrasMap := map[string]any{"status": rs.Status, "cost": rs.Cost, "status code": strconv.Itoa(rs.StatusCode)}
	extra, _ := structpb.NewStruct(extrasMap)
	if rs.StatusCode >= 500 && rs.StatusCode < 502 {

		err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent,
			&commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_UNKNOWN,
				ExternalId: rs.MessageId,
				Extras:     extra,
			})
		if err != nil {
			log.WithError(err).
				Warn("could not update status on notification service")
			return nil
		}
		return fmt.Errorf("error response from Africa's Talking server %v : %v", client.StatusCodeMap[rs.StatusCode], rs)
	}

	notificationStatus := commonv1.STATUS_QUEUED
	if rs.StatusCode >= 400 && rs.StatusCode < 500 {
		notificationStatus = commonv1.STATUS_FAILED
	}

	err = ms.eventsMan.Emit(ctx, events.NotificationStatusUpdateEvent,
		&commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_ACTIVE,
			Status:     notificationStatus,
			ExternalId: rs.MessageId,
			Extras:     extra,
		})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.WithFields(map[string]any{
		"message_id":  rs.MessageId,
		"status":      rs.Status,
		"cost":        rs.Cost,
		"status_code": rs.StatusCode}).
		Debug("Africa's Talking SMS message sent successfully")
	return nil
}
