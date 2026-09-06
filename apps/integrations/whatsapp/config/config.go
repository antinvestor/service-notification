package config

import (
	"github.com/pitabwire/frame/v2/config"
)

// WhatsAppConfig configures the WhatsApp Business Cloud API integration.
//
// Per-connection credentials (access token, phone number id, app secret, verify token)
// live in the settings service keyed by connection or route id; the env values here are
// service-wide fallbacks for single-tenant deployments.
type WhatsAppConfig struct {
	config.ConfigurationDefault

	SettingsIntegrationName string `envDefault:"WhatsApp" env:"SETTINGS_INTEGRATION_NAME"`
	SettingsIntegrationID   string `envDefault:"notification.whatsapp" env:"SETTINGS_INTEGRATION_ID"`

	ProfileServiceURI                        string `envDefault:"127.0.0.1:7005" env:"PROFILE_SERVICE_URI"`
	SettingsServiceURI                       string `envDefault:"127.0.0.1:7005" env:"SETTINGS_SERVICE_URI"`
	NotificationServiceURI                   string `envDefault:"127.0.0.1:7005" env:"NOTIFICATION_SERVICE_URI"`
	ProfileServiceWorkloadAPITargetPath      string `envDefault:"/ns/profile/sa/service-profile" env:"PROFILE_SERVICE_WORKLOAD_API_TARGET_PATH"`
	SettingsServiceWorkloadAPITargetPath     string `envDefault:"/ns/profile/sa/service-settings" env:"SETTINGS_SERVICE_WORKLOAD_API_TARGET_PATH"`
	NotificationServiceWorkloadAPITargetPath string `envDefault:"/ns/notifications/sa/service-notification" env:"NOTIFICATION_SERVICE_WORKLOAD_API_TARGET_PATH"`

	QueueWhatsAppDequeueName string `envDefault:"notifications.whatsapp.dequeue" env:"QUEUE_NOTIFICATION_WHATSAPP_DEQUEUE_NAME"`
	QueueWhatsAppDequeueURI  string `envDefault:"mem://notifications.whatsapp.de.queue" env:"QUEUE_NOTIFICATION_WHATSAPP_DEQUEUE_URI"`

	GraphAPIURL     string `envDefault:"https://graph.facebook.com" env:"WHATSAPP_GRAPH_API_URL"`
	GraphAPIVersion string `envDefault:"v21.0" env:"WHATSAPP_GRAPH_API_VERSION"`

	// AppSecret signs webhook payloads (X-Hub-Signature-256). VerifyToken answers the
	// webhook subscription handshake. Both may instead be stored per connection.
	AppSecret   string `envDefault:"" env:"WHATSAPP_APP_SECRET"`
	VerifyToken string `envDefault:"" env:"WHATSAPP_VERIFY_TOKEN"`

	RequestTimeoutSeconds int `envDefault:"30" env:"WHATSAPP_REQUEST_TIMEOUT_SECONDS"`
}
