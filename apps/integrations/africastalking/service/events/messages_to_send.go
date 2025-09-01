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
	"google.golang.org/protobuf/types/known/structpb"
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

		extrasMap := map[string]any{
			"error": err.Error(),
		}
		extra, _ := structpb.NewStruct(extrasMap)

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

		extrasMap["errcode"] = fmt.Sprintf("%v", appErr.ErrorCode())
		extra, _ = structpb.NewStruct(extrasMap)
		_, _ = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_INACTIVE,
			Status:     commonv1.STATUS_FAILED,
			ExternalId: "",
			Extras:     extra,
		})
		return nil
	}

	rs := resp.SMSMessageData.Recipients[0]

	extrasMap := map[string]any{"status": rs.Status, "": rs.Cost, "status code": strconv.Itoa(rs.StatusCode)}
	extra, _ := structpb.NewStruct(extrasMap)
	if rs.StatusCode >= 500 && rs.StatusCode < 502 {

		_, _ = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
			Id:         notification.GetId(),
			State:      commonv1.STATE_ACTIVE,
			Status:     commonv1.STATUS_UNKNOWN,
			ExternalId: rs.MessageId,
			Extras:     extra,
		})
		return fmt.Errorf("server responded in error %v : -> %v", client.StatusCodeMap[rs.StatusCode], rs)
	}

	notificationStatus := commonv1.STATUS_QUEUED
	if rs.StatusCode >= 400 && rs.StatusCode < 500 {
		notificationStatus = commonv1.STATUS_FAILED
	}

	_, err = ms.NotificationCli.Svc().StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
		Id:         notification.GetId(),
		State:      commonv1.STATE_ACTIVE,
		Status:     notificationStatus,
		ExternalId: rs.MessageId,
		Extras:     extra,
	})
	if err != nil {
		log.WithError(err).Warn("could not update status on notification service")
	}

	log.Debug("successfully sent out message")
	return nil
}
