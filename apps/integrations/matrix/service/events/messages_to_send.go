package events

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/gomatrix"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/client"
	"github.com/pitabwire/frame"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type MessageToSend struct {
	Service         *frame.Service
	ProfileCli      profilev1connect.ProfileServiceClient
	NotificationCli *notificationv1.NotificationClient
	MatrixCli       *client.Client
}

func (ms *MessageToSend) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	log := ms.Service.Log(ctx)

	notification := &notificationv1.Notification{}

	err := proto.Unmarshal(payload, notification)
	if err != nil {
		return err
	}

	resp, err := ms.MatrixCli.Send(ctx, notification)
	if err != nil {
		log = log.WithError(err)

		extraData := map[string]any{
			"error": err.Error(),
		}

		extra, _ := structpb.NewStruct(extraData)

		var respErr gomatrix.RespError
		if !errors.Is(err, respErr) {

			log.Error("could not publish message")

			_, _ = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
				Id:         notification.GetId(),
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_UNKNOWN,
				ExternalId: "",
				Extras:     extra,
			})

			return err
		}

		log.Error("server responded in error")

		extraData["errcode"] = respErr.ErrCode
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

	extraData := map[string]any{"status": "ok"}
	extra, _ := structpb.NewStruct(extraData)

	_, err = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_INACTIVE,
		Status:     commonv1.STATUS_SUCCESSFUL,
		ExternalId: resp.EventID,
		Extras:     extra,
	})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
