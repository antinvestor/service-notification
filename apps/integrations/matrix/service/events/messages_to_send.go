package events

import (
	"context"
	"errors"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/gomatrix"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/client"
	"github.com/pitabwire/frame"
	"google.golang.org/protobuf/proto"
)

type MessageToSend struct {
	Service         *frame.Service
	ProfileCli      *profilev1.ProfileClient
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

		extra := map[string]string{
			"error": err.Error(),
		}

		var respErr gomatrix.RespError
		if !errors.Is(err, respErr) {

			log.Error("could not publish message")

			_, _ = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_ACTIVE, commonv1.STATUS_UNKNOWN, "", extra)

			return err
		}

		log.Error("server responded in error")

		extra["errcode"] = respErr.ErrCode

		_, _ = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_INACTIVE, commonv1.STATUS_FAILED, "", extra)

		return nil

	}

	extra := map[string]string{"status": "ok"}

	_, err = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_INACTIVE, commonv1.STATUS_SUCCESSFUL, resp.EventID, extra)
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
