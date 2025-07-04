package config

import "github.com/pitabwire/frame"

type NotificationMatrixConfig struct {
	frame.ConfigurationDefault
	ProfileServiceURI      string `default:"127.0.0.1:7005" envconfig:"PROFILE_SERVICE_URI"`
	PartitionServiceURI    string `default:"127.0.0.1:7003" envconfig:"PARTITION_SERVICE_URI"`
	NotificationServiceURI string `default:"127.0.0.1:7005" envconfig:"NOTIFICATION_SERVICE_URI"`

	DefaultLanguageCode string `default:"en" envconfig:"DEFAULT_LANGUAGE_CODE"`

	// Matrix configuration
	QueueMatrixDequeueName string `default:"matrix.natifications.dequeue" envconfig:"QUEUE_MATRIX_NOTIFICATION_DEQUEUE_NAME"`
	QueueMatrixDequeueURI  string `default:"mem://matrix.natifications.de.queue" envconfig:"QUEUE_MATRIX_NOTIFICATION_DEQUEUE_URI"`

	MatrixServerURL   string `default:"https://stawi.im" envconfig:"MATRIX_SERVER_URL"`
	MatrixUserID      string `default:"" envconfig:"MATRIX_USER_ID"`
	MatrixAccessToken string `default:"" envconfig:"MATRIX_ACCESS_TOKEN"`

	MatrixServerDomain string `default:"stawi.im" envconfig:"MATRIX_SERVER_DOMAIN"`
}
