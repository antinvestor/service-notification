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
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/client"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/types/known/structpb"
)

type ATServer struct {
	ProfileCli        profilev1connect.ProfileServiceClient
	NotificationCli   notificationv1connect.NotificationServiceClient
	AfricasTalkingCli *client.Client
}

func NewATServer(
	profileCli profilev1connect.ProfileServiceClient,
	notificationCli notificationv1connect.NotificationServiceClient,
	africasTalkingCli *client.Client,
) *ATServer {
	return &ATServer{
		ProfileCli:        profileCli,
		NotificationCli:   notificationCli,
		AfricasTalkingCli: africasTalkingCli,
	}
}

func (ps *ATServer) writeError(ctx context.Context, w http.ResponseWriter, err error, code int) {
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

func (ps *ATServer) NewRouterV1() *http.ServeMux {
	userServeMux := http.NewServeMux()

	userServeMux.HandleFunc("/receive/notification/{routeID}", ps.ReceiveNotification)
	return userServeMux
}

func (ps *ATServer) ReceiveNotification(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	routeID := req.PathValue("routeID")

	if routeID == "" {
		// Mostly this is not valid
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode("successfully handled")
	}

	var payload map[string]any
	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		ps.writeError(ctx, rw, err, http.StatusBadRequest)
		return
	}

	rawIPData := util.GetIP(req)

	notificationCategory := ps.AfricasTalkingCli.Categorise(ctx, payload)

	var appErr *apperrors.Error
	switch notificationCategory {
	case client.DeliveryReport:
		appErr = ps.handleDeliveryReport(ctx, routeID, rawIPData, payload)

	case client.BulkSMSOptOut:
		appErr = ps.handleBulkSMSOptOut(ctx, routeID, rawIPData, payload)
	case client.SubscriptionNotifications:
		appErr = ps.handleSubscriptionNotifications(ctx, routeID, rawIPData, payload)
	case client.IncomingMessages:
		appErr = ps.handleIncomingMessages(ctx, rawIPData, routeID, payload)
	default:
		appErr = apperrors.ErrInvalidFormat.Extend("Could not determine notification category")
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
// Delivery Report notification contents
//
// Parameter
// id String
// A unique identifier for each message. This is the same id as the one in the response when a message is sent.
//
// status String
// The status of the message. Possible values are:
//
// phoneNumber String
// This is phone number that the message was sent out to.
//
// networkCode String
// A unique identifier for the telco that handled the message.
//
// failureReason String Optional
// Only provided if status is Rejected or Failed. Possible values are:
//
// retryCount Integer
// Number of times the request to send a message to the device was retried before it succeeded or definitely failed. Note: This only applies for premium SMS messages.
//

func (ps *ATServer) handleDeliveryReport(ctx context.Context, routeID, ip string, payload map[string]any) *apperrors.Error {

	internalStatus := commonv1.STATUS_UNKNOWN

	externalID := payload["id"].(string)
	status := payload["status"]
	switch status.(string) {
	case "Success":
		internalStatus = commonv1.STATUS_SUCCESSFUL
	case "Submitted", "Buffered", "AbsentSubscriber", "Sent":
		internalStatus = commonv1.STATUS_QUEUED
	case "Expired", "Failed", "Rejected":
		internalStatus = commonv1.STATUS_FAILED
	}

	extraData := map[string]any{}
	for k, v := range payload {
		extraData[k] = fmt.Sprintf("%v", v)
	}
	extraData["route"] = routeID
	extraData["ip"] = ip
	networkCode, ok := payload["networkCode"]
	if ok {
		extraData["network"] = client.SupportedNetworksMap[networkCode.(int)]
	}
	failureReason, ok := payload["failureReason"]
	if ok {
		extraData["failureReasonDetail"] = client.FailureReasonOnRejectedOrFailedMap[failureReason.(string)]
	}

	extra, _ := structpb.NewStruct(extraData)
	_, err := ps.NotificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
		Id:         "",
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

// handleBulkSMSOptOut Sent whenever a user opts out of receiving messages from your alphanumeric sender ID.
//
// To receive bulk sms opt out notifications, you need to set a bulk sms opt out callback URL. From the dashboard select SMS -> SMS Callback URLs -> Bulk SMS Opt Out.
//
// The instructions on how to opt out are automatically appended to the first message you send to the mobile subscriber. From then onwards, any other message will be sent ‘as is’ to the subscriber.
//
// Bulk sms opt out notification contents
//
// Parameter
// senderId String
// This is the shortcode/alphanumeric sender id the user opted out from.
//
// phoneNumber String
// This will contain the phone number of the subscriber who opted out.
//

func (ps *ATServer) handleBulkSMSOptOut(ctx context.Context, routeID, ip string, payload map[string]any) *apperrors.Error {

	return nil
}

// handleSubscriptionNotifications Sent whenever someone subscribes or unsubscribes from any of your premium SMS products.
//
// To receive premium sms subscription notifications, you need to set a subscription notification callback URL. From the dashboard select SMS -> SMS Callback URLs -> Subscription Notifications.
//
// # Subscription notification contents
//
// Parameter
// phoneNumber String
// Phone number to subscribe or unsubscribe.
//
// shortCode String
// The short code that has this product.
//
// keyword String
// The keyword of the product that the user has subscribed or unsubscribed from.
//
// updateType String
// The type of the update. The value could either be addition or deletion.
func (ps *ATServer) handleSubscriptionNotifications(ctx context.Context, routeID, ip string, payload map[string]any) *apperrors.Error {

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
func (ps *ATServer) handleIncomingMessages(ctx context.Context, routeID, ip string, payload map[string]any) *apperrors.Error {

	return nil
}
