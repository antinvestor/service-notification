package events

import (
	"context"
	"errors"
	"fmt"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame"
	"google.golang.org/protobuf/proto"
)

type MessageToSend struct {
	Service         *frame.Service
	ProfileCli      *profilev1.ProfileClient
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

		extra := map[string]string{
			"error": err.Error(),
		}

		var appErr *apperrors.Error
		ok := errors.As(err, &appErr)
		if !ok || appErr.IsRetriable() {

			_, _ = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_ACTIVE, commonv1.STATUS_UNKNOWN, "", extra)

			return err
		}

		extra["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		_, _ = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_INACTIVE, commonv1.STATUS_FAILED, "", extra)
		return nil
	}

	_, err = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_INACTIVE, commonv1.STATUS_SUCCESSFUL, "", map[string]string{})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
