package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/types/known/structpb"
)

type SMTPServer struct {
	ProfileCli      profilev1connect.ProfileServiceClient
	NotificationCli notificationv1connect.NotificationServiceClient
	EmailSMTPCli    *client.Client
}

func NewSMTPServer(
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	emailSMTPCli *client.Client,
) *SMTPServer {
	return &SMTPServer{
		ProfileCli:      profileCli,
		NotificationCli: notificationCli,
		EmailSMTPCli:    emailSMTPCli,
	}
}

func (ps *SMTPServer) writeError(ctx context.Context, w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")

	log := util.Log(ctx).
		WithField("code", code)

	log.WithError(err).Error("internal service error")
	w.WriteHeader(code)

	err = json.NewEncoder(w).Encode(fmt.Sprintf(" internal processing err message: %v", err))
	if err != nil {
		log.WithError(err).Error("could not write error to response")
	}
}

func (ps *SMTPServer) NewRouterV1() *http.ServeMux {
	userServeMux := http.NewServeMux()

	userServeMux.HandleFunc("/receive/notification/{routeID}", ps.ReceiveNotification)
	return userServeMux
}

func (ps *SMTPServer) ReceiveNotification(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	routeID := req.PathValue("routeID")

	if routeID == "" {
		// Mostly this is not valid
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode("successfully handled")
		return
	}

	var payload map[string]any
	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		ps.writeError(ctx, rw, err, http.StatusBadRequest)
		return
	}

	var appErr *apperrors.Error

	metadata, ok := payload["Metadata"].(map[string]any)
	if !ok {
		appErr = ps.handleIncomingMessages(ctx, routeID, payload)
	} else {
		appErr = ps.handleDeliveryReport(ctx, routeID, metadata, payload)
	}

	if appErr != nil {
		ps.writeError(ctx, rw, appErr, appErr.ErrorCode())
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode("successfully handled")
}

// handleDeliveryReport Sent whenever the mobile service provider confirms or rejects delivery of a message.
//	To receive delivery reports, you need to set a delivery report callback URL. From the dashboard select SMS -> SMS Callback URLs -> Delivery Reports.
//

func (ps *SMTPServer) handleDeliveryReport(ctx context.Context, routeID string, metadata, payload map[string]any) *apperrors.Error {

	notificationID, ok := metadata["notification-id"]
	if !ok {
		return apperrors.ErrDataNotFound.Extend("no notification id was found in metadata")
	}

	externalID, _ := payload["MessageID"].(string)

	internalStatus := commonv1.STATUS_SUCCESSFUL
	reportType, _ := payload["Type"].(string)
	switch reportType {
	case "HardBounce", "SpamComplaint":
		internalStatus = commonv1.STATUS_FAILED
	}

	extraData := map[string]any{}
	for k, v := range payload {
		extraData[k] = fmt.Sprintf("%v", v)
	}
	extraData["route"] = routeID
	extra, _ := structpb.NewStruct(extraData)

	_, err := ps.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
		Id:         fmt.Sprintf("%v", notificationID),
		State:      commonv1.STATE_INACTIVE,
		Status:     internalStatus,
		ExternalId: externalID,
		Extras:     extra,
	}))
	if err != nil {
		return apperrors.ErrSystemFailure.Extend(err.Error())
	}
	return nil
}

// handleIncomingMessages Sent whenever a message is sent to any of your registered shortcodes.
// To receive incoming messages, you need to set an incoming messages callback URL. From the dashboard select SMS -> SMS Callback URLs -> Incoming Messages.
//
// # Incoming message notification contents
//
// Parameter
// date String
// The date and time when the message was received.
//
// from String
// The number that sent the message.
//
// id String
// The internal ID that we use to store this message.
//
// linkId String Optional
// Parameter required when responding to an on-demand user request with a premium message.
//
// text String
// The message content.
//
// to String
// The number to which the message was sent.
//
// cost String:
// Amount incurred to send this sms. The format of this string is: (3-digit Currency Code)(space)(Decimal Value) e.g KES 1.00
//
// networkCode String
// A unique identifier for the telco that handled the message.
func (ps *SMTPServer) handleIncomingMessages(ctx context.Context, routeID string, payload map[string]any) *apperrors.Error {

	return nil
}
