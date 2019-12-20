package main

import (
	"log"
	"os"
	"time"

	"antinvestor.com/service/notification/service"
	"antinvestor.com/service/notification/utils"
)

func main() {

	serviceName := "Notification"

	logger, err := utils.ConfigureLogging(serviceName)
	if err != nil {
		log.Fatal("Failed to configure logging: " + err.Error())
	}

	closer, err := utils.ConfigureJuegler(serviceName)
	if err != nil {
		logger.Fatal("Failed to configure Juegler: " + err.Error())
	}

	defer closer.Close()

	database, err := utils.ConfigureDatabase(logger, false)
	if err != nil {
		logger.WithError(err).Fatal("Could not Configure write database")
	}
	defer database.Close()

	replicaDatabase, err := utils.ConfigureDatabase(logger, true)
	if err != nil {
		logger.WithError(err).Fatal("Could not Configure read database")
	}
	defer replicaDatabase.Close()

	queue, queueChecker, err := utils.ConfigureQueue(logger)
	if err != nil {
		logger.WithError(err).Fatal("Could not configure Stan queue")
	}
	defer queue.Close()

	isMigration := utils.GetEnv(utils.EnvOnlyMigrate, "")
	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration == "true" {

		logger.Info("Initiating migrations")

		service.PerformMigration(logger, database)

	} else {
		logger.Infof("Initiating the service at %v", time.Now())

		healthChecker, err := utils.ConfigureHealthChecker(logger, database, replicaDatabase, queueChecker)
		if err != nil {
			logger.Warnf("Error configuring health checks: %v", err)
		}

		env := utils.Env{
			Logger: logger,
			Health: healthChecker,
			Queue: queue,
		}

		env.SetWriteDb(database)
		env.SetReadDb(replicaDatabase)

		service.RunServer(&env)
	}

}
