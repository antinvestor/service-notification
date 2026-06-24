package events

import (
	"context"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Business metric instruments for the notification lifecycle.
//
// Every instrument is tenant-scoped transparently: frame's
// telemetry.BusinessMetrics attaches tenant_id and partition_id from the
// context's security claims on every measurement, and frame's queue layer
// propagates those claims from the originating request into event handlers.
var (
	businessMetrics = telemetry.NewBusinessMetrics("service-notification")

	notificationsQueuedTotal = businessMetrics.Counter(
		"notifications_queued_total",
		"Notifications accepted and queued for processing",
	)
	notificationsSentTotal = businessMetrics.Counter(
		"notifications_sent_total",
		"Outbound notifications dispatched to a delivery route",
	)
	notificationsDeliveredTotal = businessMetrics.Counter(
		"notifications_delivered_total",
		"Outbound notifications confirmed delivered by their route",
	)
	notificationsFailedTotal = businessMetrics.Counter(
		"notifications_failed_total",
		"Notifications that failed processing or delivery",
	)
	notificationsSendDuration = businessMetrics.Histogram(
		"notifications_send_duration_ms",
		"Time from notification creation to dispatch on a delivery route",
	)
)

const unknownAttrValue = "unknown"

func notificationChannel(n *models.Notification) string {
	if n == nil || n.NotificationType == "" {
		return unknownAttrValue
	}
	return n.NotificationType
}

func failureReason(nStatus *models.NotificationStatus) string {
	if reason := nStatus.Extra.GetString("step"); reason != "" {
		return reason
	}
	return unknownAttrValue
}

// recordStatusMetrics translates a persisted notification status transition
// into business metrics. NotificationStatusSave is the single sink through
// which every lifecycle transition flows, so recording here covers queueing,
// dispatch, delivery confirmations and failures from all paths.
func recordStatusMetrics(ctx context.Context, n *models.Notification, nStatus *models.NotificationStatus) {
	channelAttr := attribute.String("channel", notificationChannel(n))

	switch commonv1.STATUS(nStatus.Status) {
	case commonv1.STATUS_QUEUED:
		// Re-queue transitions (routing, release) carry STATE_ACTIVE; only
		// count first acceptance so each notification is queued once.
		if commonv1.STATE(nStatus.State) != commonv1.STATE_CREATED {
			return
		}
		notificationsQueuedTotal.Add(ctx, 1, channelAttr)
	case commonv1.STATUS_IN_PROCESS:
		if !n.OutBound {
			return
		}
		attrs := []attribute.KeyValue{channelAttr}
		if n.TemplateID != "" {
			attrs = append(attrs, attribute.String("template", n.TemplateID))
		}
		notificationsSentTotal.Add(ctx, 1, attrs...)
		notificationsSendDuration.Record(ctx, float64(time.Since(n.CreatedAt).Milliseconds()), channelAttr)
	case commonv1.STATUS_SUCCESSFUL:
		if !n.OutBound {
			return
		}
		notificationsDeliveredTotal.Add(ctx, 1, channelAttr)
	case commonv1.STATUS_FAILED:
		notificationsFailedTotal.Add(ctx, 1, channelAttr, attribute.String("reason", failureReason(nStatus)))
	default:
		// Other statuses are not lifecycle transitions we report on.
	}
}
