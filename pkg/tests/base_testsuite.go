package tests

import (
	"context"
	"testing"

	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	"github.com/pitabwire/frame/v2/frametests"
	"github.com/pitabwire/frame/v2/frametests/definition"
	"github.com/pitabwire/frame/v2/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
)

const PostgresqlDBImage = "paradedb/paradedb:latest"

const (
	DefaultRandomStringLength = 8
)

type BaseTestSuite struct {
	frametests.FrameBaseTestSuite
}

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_notification", definition.WithUserName("ant"), definition.WithCredential("s3cr3t"))
	resources := []definition.TestResource{pg}
	return resources
}

func (bs *BaseTestSuite) SetupSuite() {
	bs.InitResourceFunc = initResources
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) GetProfileCli(_ context.Context) profilev1connect.ProfileServiceClient {
	// For now, return nil as we don't need actual profile client for basic tests
	return nil
}

func (bs *BaseTestSuite) GetTenancyCli(_ context.Context) tenancyv1connect.TenancyServiceClient {
	// For now, return nil as we don't need actual partition client for basic tests
	return nil
}

func (bs *BaseTestSuite) TearDownSuite() {
	bs.FrameBaseTestSuite.TearDownSuite()
}

// WithTestDependancies Creates subtests with each known DependancyOption.
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *definition.DependencyOption)) {
	options := []*definition.DependencyOption{
		definition.NewDependancyOption("default", util.RandomAlphaNumericString(DefaultRandomStringLength), bs.Resources()),
	}

	frametests.WithTestDependencies(t, options, testFn)
}
