package events

import (
	"context"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func metricAttrSets(rm metricdata.ResourceMetrics, name string) []attribute.Set {
	var out []attribute.Set
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					out = append(out, dp.Attributes)
				}
			case metricdata.Histogram[float64]:
				for _, dp := range d.DataPoints {
					out = append(out, dp.Attributes)
				}
			}
		}
	}
	return out
}

func requireAttr(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()
	v, ok := set.Value(attribute.Key(key))
	require.True(t, ok, "attribute %q must be present", key)
	require.Equal(t, want, v.AsString(), "attribute %q", key)
}

// Notification lifecycle counters must transparently carry tenant_id and
// partition_id from the request context claims alongside their business
// attributes.
func TestRecordStatusMetricsTenantScoped(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	claims := &security.AuthenticationClaims{TenantID: "tenant-x", PartitionID: "part-y"}
	claims.Subject = "user-tenant-x"
	ctx := claims.ClaimsToContext(context.Background())

	n := &models.Notification{
		NotificationType: "sms",
		OutBound:         true,
		TemplateID:       "tmpl-1",
	}
	n.CreatedAt = time.Now().Add(-250 * time.Millisecond)

	recordStatusMetrics(ctx, n, &models.NotificationStatus{
		State:  int32(commonv1.STATE_CREATED.Number()),
		Status: int32(commonv1.STATUS_QUEUED.Number()),
	})
	// Re-queue after routing must not double count acceptance.
	recordStatusMetrics(ctx, n, &models.NotificationStatus{
		State:  int32(commonv1.STATE_ACTIVE.Number()),
		Status: int32(commonv1.STATUS_QUEUED.Number()),
	})
	recordStatusMetrics(ctx, n, &models.NotificationStatus{
		Status: int32(commonv1.STATUS_IN_PROCESS.Number()),
	})
	recordStatusMetrics(ctx, n, &models.NotificationStatus{
		Status: int32(commonv1.STATUS_SUCCESSFUL.Number()),
	})
	recordStatusMetrics(ctx, n, &models.NotificationStatus{
		Status: int32(commonv1.STATUS_FAILED.Number()),
		Extra:  data.JSONMap{"step": "publish_to_queue"},
	})

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	for _, name := range []string{
		"notifications_queued_total",
		"notifications_sent_total",
		"notifications_delivered_total",
		"notifications_failed_total",
		"notifications_send_duration_ms",
	} {
		sets := metricAttrSets(rm, name)
		require.Len(t, sets, 1, "%s must have exactly one datapoint", name)
		requireAttr(t, sets[0], "tenant_id", "tenant-x")
		requireAttr(t, sets[0], "partition_id", "part-y")
		requireAttr(t, sets[0], "channel", "sms")
	}

	sentSets := metricAttrSets(rm, "notifications_sent_total")
	requireAttr(t, sentSets[0], "template", "tmpl-1")

	failedSets := metricAttrSets(rm, "notifications_failed_total")
	requireAttr(t, failedSets[0], "reason", "publish_to_queue")
}
