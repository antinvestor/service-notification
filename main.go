package main

import (
	
	"log"
	"os"
	"time"

	"bitbucket.org/antinvestor/service-notification/service"
	"bitbucket.org/antinvestor/service-notification/utils"
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
		logger.Warnf("Configuring write database has error: %v", err)
	}

	replicaDatabase, err := utils.ConfigureDatabase(logger, true)
	if err != nil {
		logger.Warnf("Configuring read only database has error: %v", err)
	}



	stdArgs := os.Args[1:]

	if len(stdArgs) > 0 && stdArgs[0] == "client" {

		logger.Infof("Initiating the client at %v", time.Now())
		service.Runclient(database)

	}else{

		isMigration := utils.GetEnv(utils.EnvOnlyMigrate, "")
		stdArgs := os.Args[1:]
		if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration == "true" {

			logger.Info("Initiating migrations")

		service.PerformMigration(logger, database)

	} else {
		logger.Infof("Initiating the service at %v", time.Now())
		
		healthChecker, err := utils.ConfigureHealthChecker(logger, database, replicaDatabase)
		if err != nil {
			logger.Warnf("Error configuring health checks: %v", err)
		}

		env := service.Env{
			Logger:     logger,
			Health:     healthChecker,
			ServerPort: utils.GetEnv("SERVER_PORT", "7000"),
		}
		 
		env.SetWriteDb(database)
		env.SetReadDb(replicaDatabase)

		service.RunServer(&env)
	}
	
	}

}
