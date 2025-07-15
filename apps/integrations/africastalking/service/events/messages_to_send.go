package events

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame"
	"google.golang.org/protobuf/proto"
)

type MessageToSend struct {
	Service           *frame.Service
	ProfileCli        *profilev1.ProfileClient
	NotificationCli   *notificationv1.NotificationClient
	AfricasTalkingCli *client.Client
}

func (ms *MessageToSend) Handle(ctx context.Context, headers map[string]string, payload []byte) error {

	log := ms.Service.Log(ctx)

	notification := &notificationv1.Notification{}

	err := proto.Unmarshal(payload, notification)
	if err != nil {
		return err
	}

	resp, err := ms.AfricasTalkingCli.Send(ctx, headers, notification)
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

	rs := resp.SMSMessageData.Recipients[0]

	extra := map[string]string{"status": rs.Status, "": rs.Cost, "status code": strconv.Itoa(rs.StatusCode)}

	if rs.StatusCode >= 500 && rs.StatusCode < 502 {

		_, _ = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_ACTIVE, commonv1.STATUS_UNKNOWN, rs.MessageId, extra)
		return fmt.Errorf("server responded in error %v : -> %v", client.StatusCodeMap[rs.StatusCode], rs)
	}

	notificationStatus := commonv1.STATUS_QUEUED
	if rs.StatusCode >= 400 && rs.StatusCode < 500 {
		notificationStatus = commonv1.STATUS_FAILED
	}

	_, err = ms.NotificationCli.UpdateStatus(ctx, notification.GetId(), commonv1.STATE_INACTIVE, notificationStatus, rs.MessageId, extra)
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
