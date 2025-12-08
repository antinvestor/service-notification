package config

import (
	"github.com/pitabwire/frame/config"
)

type EmailSMTPConfig struct {
	config.ConfigurationDefault

	SettingsIntegrationName string `envDefault:"Email SMTP" env:"SETTINGS_INTEGRATION_NAME"`
	SettingsIntegrationID   string `envDefault:"notification.emailsmtp" env:"SETTINGS_INTEGRATION_ID"`

	ProfileServiceURI      string `envDefault:"127.0.0.1:7005" env:"PROFILE_SERVICE_URI"`
	SettingsServiceURI     string `envDefault:"127.0.0.1:7005" env:"SETTINGS_SERVICE_URI"`
	PartitionServiceURI    string `envDefault:"127.0.0.1:7003" env:"PARTITION_SERVICE_URI"`
	NotificationServiceURI string `envDefault:"127.0.0.1:7005" env:"NOTIFICATION_SERVICE_URI"`

	// Africans talking configuration
	QueueATDequeueName string `envDefault:"natifications.emailsmtp.dequeue" env:"QUEUE_NOTIFICATION_EMAIL_DEQUEUE_NAME"`
	QueueATDequeueURI  string `envDefault:"mem://natifications.email.de.queue" env:"QUEUE_NOTIFICATION_EMAIL_DEQUEUE_URI"`

	SMTPServerHOST      string `envDefault:"smtp.postmarkapp.com" env:"SMTP_SERVER_HOST"`
	SMTPServerPORT      int    `envDefault:"587" env:"SMTP_SERVER_PORT"`
	SMTPServerAccessKey string `envDefault:"" env:"SMTP_SERVER_ACCESS_KEY"`
	SMTPServerSecretKey string `envDefault:"" env:"SMTP_SERVER_SECRET_KEY"`
}
