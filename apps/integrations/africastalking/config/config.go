package config

import (
	"github.com/pitabwire/frame/config"
)

type AfricasTalkingConfig struct {
	config.ConfigurationDefault

	SettingsIntegrationName string `envDefault:"Africastalking" env:"SETTINGS_INTEGRATION_NAME"`
	SettingsIntegrationID   string `envDefault:"Default" env:"SETTINGS_INTEGRATION_ID"`

	ProfileServiceURI      string `envDefault:"127.0.0.1:7005" env:"PROFILE_SERVICE_URI"`
	SettingsServiceURI     string `envDefault:"127.0.0.1:7005" env:"SETTINGS_SERVICE_URI"`
	PartitionServiceURI    string `envDefault:"127.0.0.1:7003" env:"PARTITION_SERVICE_URI"`
	NotificationServiceURI string `envDefault:"127.0.0.1:7005" env:"NOTIFICATION_SERVICE_URI"`

	// Africans talking configuration
	QueueATDequeueName string `envDefault:"africastalking.natifications.dequeue" env:"QUEUE_NOTIFICATION_AFRICASTALKING_DEQUEUE_NAME"`
	QueueATDequeueURI  string `envDefault:"mem://africastalking.natifications.de.queue" env:"QUEUE_NOTIFICATION_AFRICASTALKING_DEQUEUE_URI"`

	ATServerURL string `envDefault:"https://api.africastalking.com/version1/messaging/bulk" env:"AT_SERVER_URL"`
}
