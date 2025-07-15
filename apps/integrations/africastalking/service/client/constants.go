package client

const (
	DeliveryReport            = "Delivery Report"
	IncomingMessages          = "Incoming messages"
	BulkSMSOptOut             = "Bulk SMS Opt Out"
	SubscriptionNotifications = "Subscription Notifications"
)

var SupportedNetworksMap = map[int]string{
	62120: "Airtel Nigeria",
	62130: "MTN Nigeria",
	62150: "Glo Nigeria",
	62160: "Etisalat Nigeria",
	63510: "MTN Rwanda",
	63513: "Tigo Rwanda",
	63514: "Airtel Rwanda",
	63902: "Safaricom",
	63903: "Airtel Kenya",
	63907: "Orange Kenya",
	63999: "Equitel Kenya",
	64002: "Tigo Tanzania",
	64003: "Zantel Tanzania",
	64004: "Vodacom Tanzania",
	64005: "Airtel Tanzania",
	64007: "TTCL Tanzania",
	64009: "Halotel Tanzania",
	64101: "Airtel Uganda",
	64110: "MTN Uganda",
	64111: "UTL Uganda",
	64114: "Africell Uganda",
	65001: "TNM Malawi",
	65010: "Airtel Malawi",
	99999: "Athena (Custom networkCode for the sandbox environment).",
}

var StatusMap = map[string]string{
	"Sent":             "The message has successfully been sent by our network.",
	"Submitted":        "The message has successfully been submitted to the MSP (Mobile Service Provider).",
	"Buffered":         "The message has been queued by the MSP.",
	"Rejected":         "The message has been rejected by the MSP. This is a final status.",
	"Success":          "The message has successfully been delivered to the receiver’s handset. This is a final status.",
	"Failed":           "The message could not be delivered to the receiver’s handset. This is a final status.",
	"AbsentSubscriber": "The message was not delivered since user’s SIM card was not reachable on the network either phone was off or in a place with no network coverage.",
	"Expired":          "The message was discarded by the telco as it was flagged, either some content in the message or the sender ID use was flagged on their firewall.",
}

var StatusCodeMap = map[int]string{
	100: "Processed",
	101: "Sent",
	102: "Queued",
	401: "RiskHold",
	402: "InvalidSenderId",
	403: "InvalidPhoneNumber",
	404: "UnsupportedNumberType",
	405: "InsufficientBalance",
	406: "UserInBlacklist",
	407: "CouldNotRoute",
	409: "DoNotDisturbRejection",
	500: "InternalServerError",
	501: "GatewayError",
	502: "RejectedByGateway",
}

var FailureReasonOnRejectedOrFailedMap = map[string]string{
	"InsufficientCredit":         "This occurs when the subscriber doesn’t have enough airtime for a premium subscription service/message",
	"InvalidLinkId":              "This occurs when a message is sent with an invalid linkId for an onDemand service",
	"UserIsInactive":             "This occurs when the subscriber is inactive or the account deactivated by the MSP (Mobile Service Provider).",
	"UserInBlackList":            "This occurs if the user has been blacklisted not to receive messages from a particular service (shortcode or keyword)",
	"UserAccountSuspended":       "This occurs when the mobile subscriber has been suspended by the MSP.",
	"NotNetworkSubcriber":        "This occurs when the message is passed to an MSP where the subscriber doesn’t belong.",
	"UserNotSubscribedToProduct": "This occurs when the message from a subscription product is sent to a phone number that has not subscribed to the product.",
	"UserDoesNotExist":           "This occurs when the message is sent to a non-existent mobile number.",
	"DeliveryFailure":            "This occurs when message delivery fails for any reason not listed above or where the MSP didn’t provide a delivery failure reason.",
	"DoNotDisturbRejection":      "Note: This only applies to Nigeria. When attempting to send an SMS message with a promotional sender ID outside the allowed time window(8pm-8am), the API will return an HTTP 409 status code, indicating a conflict. This error code signifies that the request conflicts with the predefined time restrictions for promotional sender IDs by the NCC.",
}
