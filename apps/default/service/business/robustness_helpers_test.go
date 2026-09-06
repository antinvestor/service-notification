package business_test

import (
	"testing"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/tests"
	"github.com/pitabwire/frame/v2/data"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	require.NoError(t, err)
	return s
}

// findChild returns the notification whose ParentID is parentID, or nil.
func findChild(t *testing.T, resources *tests.ServiceResources, parentID string) *models.Notification {
	t.Helper()
	query := data.NewSearchQuery(data.WithSearchFiltersAndByValue(map[string]any{"parent_id = ?": parentID}))
	results, err := resources.NotificationRepo.Search(t.Context(), query)
	require.NoError(t, err)
	for {
		res, ok := results.ReadResult(t.Context())
		if !ok {
			return nil
		}
		require.False(t, res.IsError(), "search failed: %v", res.Error())
		if items := res.Item(); len(items) > 0 {
			return items[0]
		}
	}
}
