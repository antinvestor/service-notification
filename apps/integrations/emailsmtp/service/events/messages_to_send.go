package events

import (
	"context"
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type MessageToSend struct {
	ProfileCli      profilev1connect.ProfileServiceClient
	NotificationCli notificationv1connect.NotificationServiceClient
	EmailSMTPCli    *client.Client
}

func NewMessageToSend(
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	emailSMTPCli *client.Client,
) *MessageToSend {
	return &MessageToSend{
		ProfileCli:      profileCli,
		NotificationCli: notificationCli,
		EmailSMTPCli:    emailSMTPCli,
	}
}

func (ms *MessageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := util.Log(ctx)

	notification := &notificationv1.Notification{}

	err := proto.Unmarshal(payload, notification)
	if err != nil {
		return err
	}

	err = ms.EmailSMTPCli.Send(ctx, headers, notification)
	if err != nil {
		log.WithError(err).Error("server responded in error")

		extraData := map[string]any{
			"error": err.Error(),
		}
		extra, _ := structpb.NewStruct(extraData)

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

		extraData["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extraData)

		_, _ = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_INACTIVE,
			Status:     commonv1.STATUS_FAILED,
			ExternalId: "",
			Extras:     extra,
		}))
		return nil
	}

	_, err = ms.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_INACTIVE,
		Status:     commonv1.STATUS_SUCCESSFUL,
		ExternalId: "",
	}))
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
