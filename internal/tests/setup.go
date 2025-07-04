package tests

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcNats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Database configuration

	// PostgresqlDbImage PostgreSQL Image
	PostgresqlDbImage = "postgres:17"

	DBUser     = "notification_test"
	DBPassword = "notification_test"
	DBName     = "notification_test"

	// NATS configuration

	NatsImage = "nats-streaming:0.25.5"

	NatsPort    = "4222"
	NatsUser    = "notification_test"
	NatsPass    = "notification_test"
	NatsCluster = "notification_test"

	// DefaultTestTimeout Default test timeout
	DefaultTestTimeout = 30 * time.Second
)

const (
	DefaultDB    = "postgres"
	DefaultCache = "redis"
	DefaultQueue = "nats"
)

type DependancyOption struct {
	name  string
	db    string
	cache string
	queue string
}

func (opt *DependancyOption) Name() string {
	return opt.name
}
func (opt *DependancyOption) Database() string {
	if opt.db == "" {
		return DefaultDB
	}
	return opt.db
}
func (opt *DependancyOption) Cache() string {
	if opt.cache == "" {
		return DefaultCache
	}
	return opt.cache
}
func (opt *DependancyOption) Queue() string {
	if opt.queue == "" {
		return DefaultQueue
	}
	return opt.queue
}

// setupTestResources initializes the testing resources (databases, queues) that will be shared across tests
func (s *NotificationTestSuite) setupTestResources(ctx context.Context) error {

	var setupErr error

	// Set up test containers
	s.postgresContainer, setupErr = setupPostgres(ctx)
	if setupErr != nil {
		return setupErr
	}
	s.natsContainer, setupErr = setupNats(ctx)
	if setupErr != nil {
		return setupErr
	}

	return nil
}

// setupPostgres creates a PostgreSQL testcontainer and returns the container and connection string
func setupPostgres(ctx context.Context) (*tcPostgres.PostgresContainer, error) {
	slog.Info("Setting up PostgreSQL container...")

	pgContainer, err := tcPostgres.Run(ctx, PostgresqlDbImage,
		tcPostgres.WithDatabase(DBName),
		tcPostgres.WithUsername(DBUser),
		tcPostgres.WithPassword(DBPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	return pgContainer, nil
}

// setupNats creates a NATS testcontainer and returns the container and connection string
func setupNats(ctx context.Context) (*tcNats.NATSContainer, error) {

	natsqContainer, err := tcNats.Run(ctx, NatsImage,
		tcNats.WithUsername(NatsUser),
		tcNats.WithPassword(NatsPass),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start nats container: %w", err)
	}

	return natsqContainer, nil
}

// WithTestDependancies Creates subtests with each known DependancyOption
func WithTestDependancies(t *testing.T, testFn func(t *testing.T, db DependancyOption)) {
	options := []DependancyOption{
		{
			name:  "Default",
			db:    DefaultDB,
			cache: DefaultCache,
			queue: DefaultQueue,
		},
	}
	for _, opt := range options {
		t.Run(opt.Name(), func(tt *testing.T) {
			// Removed tt.Parallel() as it conflicts with t.Setenv() used in GetService
			testFn(tt, opt)
		})
	}
}

// cleanupTestResources cleans up the test resources
func (s *NotificationTestSuite) cleanupTestResources(ctx context.Context) {
	if s.postgresContainer != nil {
		if err := s.postgresContainer.Terminate(ctx); err != nil {
			slog.Error("Failed to terminate postgres container", "error", err)
		}
	}

	if s.natsContainer != nil {
		if err := s.natsContainer.Terminate(ctx); err != nil {
			slog.Error("Failed to terminate nats container", "error", err)
		}
	}
}
