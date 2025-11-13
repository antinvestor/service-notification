package events

import (
	"context"
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type MessageToSend struct {
	Service         *frame.Service
	ProfileCli      profilev1connect.ProfileServiceClient
	NotificationCli *notificationv1.NotificationClient
	EmailSMTPCli    *client.Client
}

func (ms *MessageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := ms.Service.Log(ctx)

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

			_, _ = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_UNKNOWN,
				ExternalId: "",
				Extras:     extra,
			})

			return err
		}

		extraData["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extraData)

		_, _ = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_INACTIVE,
			Status:     commonv1.STATUS_FAILED,
			ExternalId: "",
			Extras:     extra,
		})
		return nil
	}

	_, err = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_INACTIVE,
		Status:     commonv1.STATUS_SUCCESSFUL,
		ExternalId: "",
	})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
