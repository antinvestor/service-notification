package config

import (
	"github.com/pitabwire/frame/config"
)

type UssdConfig struct {
	config.ConfigurationDefault

	ProfileServiceURI                        string `envDefault:"127.0.0.1:7005" env:"PROFILE_SERVICE_URI"`
	TenancyServiceURI                        string `envDefault:"127.0.0.1:7003" env:"TENANCY_SERVICE_URI"`
	NotificationServiceURI                   string `envDefault:"127.0.0.1:7005" env:"NOTIFICATION_SERVICE_URI"`
	SettingsServiceURI                       string `envDefault:"127.0.0.1:7005" env:"SETTINGS_SERVICE_URI"`
	ProfileServiceWorkloadAPITargetPath      string `envDefault:"/ns/profile/sa/service-profile" env:"PROFILE_SERVICE_WORKLOAD_API_TARGET_PATH"`
	TenancyServiceWorkloadAPITargetPath      string `envDefault:"/ns/auth/sa/service-tenancy" env:"TENANCY_SERVICE_WORKLOAD_API_TARGET_PATH"`
	NotificationServiceWorkloadAPITargetPath string `envDefault:"/ns/notifications/sa/service-notification" env:"NOTIFICATION_SERVICE_WORKLOAD_API_TARGET_PATH"`
	SettingsServiceWorkloadAPITargetPath     string `envDefault:"/ns/profile/sa/service-settings" env:"SETTINGS_SERVICE_WORKLOAD_API_TARGET_PATH"`

	DefaultLanguageCode  string `envDefault:"en" env:"DEFAULT_LANGUAGE_CODE"`
	SessionExpiryMinutes int    `envDefault:"5" env:"USSD_SESSION_EXPIRY_MINUTES"`
}
