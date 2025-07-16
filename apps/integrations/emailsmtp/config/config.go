package config

import "github.com/pitabwire/frame"

type EmailSMTPConfig struct {
	frame.ConfigurationDefault

	SettingsIntegrationName string `default:"Africastalking" envconfig:"SETTINGS_INTEGRATION_NAME"`
	SettingsIntegrationID   string `default:"Default" envconfig:"SETTINGS_INTEGRATION_ID"`

	ProfileServiceURI      string `default:"127.0.0.1:7005" envconfig:"PROFILE_SERVICE_URI"`
	SettingsServiceURI     string `default:"127.0.0.1:7005" envconfig:"SETTINGS_SERVICE_URI"`
	PartitionServiceURI    string `default:"127.0.0.1:7003" envconfig:"PARTITION_SERVICE_URI"`
	NotificationServiceURI string `default:"127.0.0.1:7005" envconfig:"NOTIFICATION_SERVICE_URI"`

	// Africans talking configuration
	QueueATDequeueName string `default:"natifications.emailsmtp.dequeue" envconfig:"QUEUE_NOTIFICATION_EMAIL_DEQUEUE_NAME"`
	QueueATDequeueURI  string `default:"mem://natifications.email.de.queue" envconfig:"QUEUE_NOTIFICATION_EMAIL_DEQUEUE_URI"`

	SMTPServerHOST     string `default:"smtp.postmarkapp.com" envconfig:"SMTP_SERVER_HOST"`
	SMTPServerPORT     int    `default:"587" envconfig:"SMTP_SERVER_PORT"`
	SMTPServerUserName string `default:"" envconfig:"SMTP_SERVER_USER_NAME"`
	SMTPServerPassword string `default:"" envconfig:"SMTP_SERVER_PASSWORD"`
}
