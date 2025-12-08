package events

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type messageToSend struct {
	ProfileCli        profilev1connect.ProfileServiceClient
	NotificationCli   notificationv1connect.NotificationServiceClient
	AfricasTalkingCli *client.Client
}

func NewMessageToSend(
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	africasTalkingCli *client.Client,
) queue.SubscribeWorker {
	return &messageToSend{
		ProfileCli:        profileCli,
		NotificationCli:   notificationCli,
		AfricasTalkingCli: africasTalkingCli,
	}
}

func (ms *messageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := util.Log(ctx)

	notification := &notificationv1.Notification{}

	err := proto.Unmarshal(payload, notification)
	if err != nil {
		return err
	}

	log.WithField("notification_id", notification.GetId()).
		WithField("recipient", notification.GetRecipient().GetProfileId()).
		WithField("sender", notification.GetSource().GetProfileId()).
		WithField("message_length", len(notification.GetData())).
		Debug("Sending AfricasTalking SMS message")

	resp, err := ms.AfricasTalkingCli.Send(ctx, headers, notification)
	if err != nil {
		log.WithError(err).Error("AfricasTalking API responded with error")

		extrasMap := map[string]any{
			"error": err.Error(),
		}
		extra, _ := structpb.NewStruct(extrasMap)

		var appErr *apperrors.Error
		ok := errors.As(err, &appErr)
		if !ok || appErr.IsRetriable() {

			_, _ = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_UNKNOWN,
				ExternalId: "",
				Extras:     extra,
			}))

			return err
		}

		extrasMap["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extrasMap)
		_, _ = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_INACTIVE,
			Status:     commonv1.STATUS_FAILED,
			ExternalId: "",
			Extras:     extra,
		}))
		return nil
	}

	rs := resp.SMSMessageData.Recipients[0]

	extrasMap := map[string]any{"status": rs.Status, "cost": rs.Cost, "status code": strconv.Itoa(rs.StatusCode)}
	extra, _ := structpb.NewStruct(extrasMap)
	if rs.StatusCode >= 500 && rs.StatusCode < 502 {

		_, _ = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_ACTIVE,
			Status:     commonv1.STATUS_UNKNOWN,
			ExternalId: rs.MessageId,
			Extras:     extra,
		}))
		return fmt.Errorf("AfricasTalking server responded with error %v : %v", client.StatusCodeMap[rs.StatusCode], rs)
	}

	notificationStatus := commonv1.STATUS_QUEUED
	if rs.StatusCode >= 400 && rs.StatusCode < 500 {
		notificationStatus = commonv1.STATUS_FAILED
	}

	_, err = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_ACTIVE,
		Status:     notificationStatus,
		ExternalId: rs.MessageId,
		Extras:     extra,
	}))
	if err != nil {
		log.WithError(err).WithField("notification_id", notification.GetId()).Warn("could not update status on notification service")
	}

	log.WithField("notification_id", notification.GetId()).
		WithField("message_id", rs.MessageId).
		WithField("status", rs.Status).
		WithField("cost", rs.Cost).
		WithField("status_code", rs.StatusCode).
		Debug("AfricasTalking SMS message sent successfully")
	return nil
}
