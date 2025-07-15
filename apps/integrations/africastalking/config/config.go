package config

import "github.com/pitabwire/frame"

type AfricasTalkingConfig struct {
	frame.ConfigurationDefault

	SettingsIntegrationName string `default:"Africastalking" envconfig:"SETTINGS_INTEGRATION_NAME"`
	SettingsIntegrationID   string `default:"Default" envconfig:"SETTINGS_INTEGRATION_ID"`

	ProfileServiceURI      string `default:"127.0.0.1:7005" envconfig:"PROFILE_SERVICE_URI"`
	SettingsServiceURI     string `default:"127.0.0.1:7005" envconfig:"SETTINGS_SERVICE_URI"`
	PartitionServiceURI    string `default:"127.0.0.1:7003" envconfig:"PARTITION_SERVICE_URI"`
	NotificationServiceURI string `default:"127.0.0.1:7005" envconfig:"NOTIFICATION_SERVICE_URI"`

	// Africas talking configuration
	QueueATDequeueName string `default:"africastalking.natifications.dequeue" envconfig:"QUEUE_NOTIFICATION_AFRICASTALKING_DEQUEUE_NAME"`
	QueueATDequeueURI  string `default:"mem://africastalking.natifications.de.queue" envconfig:"QUEUE_NOTIFICATION_AFRICASTALKING_DEQUEUE_URI"`

	ATServerURL string `default:"https://api.africastalking.com/version1/messaging/bulk" envconfig:"AT_SERVER_URL"`
}
