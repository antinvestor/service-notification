package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ensureDatabaseExists checks if a specific database exists and creates it if it does not.
func ensureDatabaseExists(ctx context.Context, postgresUri *url.URL, newDbName string) (*url.URL, error) {
	connectionString := postgresUri.String()
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return postgresUri, err
	}
	cfg.MaxConns = 20 // Increase pool size for concurrency
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return postgresUri, err
	}

	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return postgresUri, err
	}

	// Check if database exists before trying to create it
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s;`, newDbName))
	if err != nil {

		var pgErr *pgconn.PgError
		ok := errors.As(err, &pgErr)
		if !ok || (pgErr.Code != "42P04" && pgErr.Code != "23505" && (pgErr.Code != "XX000" || !strings.Contains(pgErr.Message, "tuple concurrently updated"))) {
			return postgresUri, err
		}
	}

	dbUserName := postgresUri.User.Username()
	_, err = pool.Exec(ctx, fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, newDbName, dbUserName))
	if err != nil {
		var pgErr *pgconn.PgError
		ok := errors.As(err, &pgErr)
		if !ok || pgErr.Code != "XX000" || !strings.Contains(pgErr.Message, "tuple concurrently updated") {
			return postgresUri, err
		}
	}

	postgresUri.Path = newDbName
	return postgresUri, nil
}

func clearDatabase(ctx context.Context, connectionString string) error {

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return err
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err != nil {
		return err
	}
	return nil
}

func generateNewDBName(randomnesPrefix string) (string, error) {
	// we cannot use 'matrix_test' here else 2x concurrently running packages will try to use the same db.
	// instead, hash the current working directory, snaffle the first 16 bytes and append that to matrix_test
	// and use that as the unique db name. We do this because packages are per-directory hence by hashing the
	// working (test) directory we ensure we get a consistent hash and don't hash against concurrent packages.
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(wd))
	databaseName := fmt.Sprintf("notifications_test_%s_%s", randomnesPrefix, hex.EncodeToString(hash[:16]))
	return databaseName, nil
}

// prepareDatabaseConnection Prepare a postgres connection string for testing.
// Returns the connection string to use and a close function which must be called when the test finishes.
// Calling this function twice will return the same database, which will have data from previous tests
// unless close() is called.
func (s *NotificationTestSuite) prepareDatabaseConnection(ctx context.Context, randomnesPrefix string, testOpts DependancyOption) (postgresDataSource string, close func(context.Context), err error) {

	if testOpts.Database() != DefaultDB {
		return "", func(ctx context.Context) {}, errors.New("only postgresql is the supported database for now")
	}

	postgresUriStr, err := s.postgresContainer.ConnectionString(ctx)
	if err != nil {
		return "", func(ctx context.Context) {}, fmt.Errorf("failed to get postgres connection string: %w", err)
	}

	parsedPostgresUri, err := url.Parse(postgresUriStr)
	if err != nil {
		return "", func(ctx context.Context) {}, err
	}

	newDatabaseName, err := generateNewDBName(randomnesPrefix)
	if err != nil {
		return "", func(ctx context.Context) {}, err
	}

	connectionUri, err := ensureDatabaseExists(ctx, parsedPostgresUri, newDatabaseName)
	if err != nil {
		return "", func(ctx context.Context) {}, err
	}

	postgresUriStr = connectionUri.String()
	return postgresUriStr, func(ctx context.Context) {
		_ = clearDatabase(ctx, postgresUriStr)
	}, nil
}
