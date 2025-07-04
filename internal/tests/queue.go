package tests

import (
	"context"
	"fmt"
	"net/url"
)

// const NatsImage = "nats:2.10"
//
// func setupNats(ctx context.Context) (*tcNats.NATSContainer, error) {
//	return tcNats.Run(ctx, NatsImage)
// }

// prepareQueueConnection Prepare a nats connection string for testing.
// Returns the connection string to use and a close function which must be called when the test finishes.
// Calling this function twice will return the same connection, which will have data from previous tests
// unless close() is called.
// func prepareQueueConnection(ctx context.Context) (dsConnection config.DataSource, close func(), err error) {
//
//	container, err := setupNats(ctx)
//	if err != nil {
//		return "", nil, err
//	}
//
//	connStr, err := container.DS(ctx)
//	if err != nil {
//		return "", nil, err
//	}
//
//	return config.DataSource(connStr), func() {
//
//		err = testcontainers.TerminateContainer(container)
//		if err != nil {
//			util.Log(ctx).WithError(err).Error("failed to terminate container")
//		}
//
//	}, nil
// }

// prepareQueueConnection Prepare a nats connection string for testing.
// Returns the connection string to use and a close function which must be called when the test finishes.
// Calling this function twice will return the same database, which will have data from previous tests
// unless close() is called.
func (s *NotificationTestSuite) prepareQueueConnection(ctx context.Context, _ string, testOpts DependancyOption) (connStr string, close func(ctx context.Context), err error) {

	if testOpts.Queue() != DefaultQueue {
		return "", func(ctx context.Context) {}, fmt.Errorf(" %s is unsupported, only nats is the supported queue for now", testOpts.Queue())
	}

	natsUriStr, err := s.natsContainer.ConnectionString(ctx)
	if err != nil {
		return "", func(ctx context.Context) {}, fmt.Errorf("failed to get postgres connection string: %w", err)
	}

	parsedNatsUri, err := url.Parse(natsUriStr)
	if err != nil {
		return "", func(ctx context.Context) {}, err
	}

	natsUriStr = parsedNatsUri.String()

	return natsUriStr, func(ctx context.Context) {
	}, nil
}
