package utils

const EnvServerPort = "SERVER_PORT"

const EnvDatabaseDriver = "DATABASE_DRIVER"
const EnvDatabaseUrl = "DATABASE_URL"
const EnvReplicaDatabaseUrl = "REPLICA_DATABASE_URL"

const EnvOnlyMigrate = "ONLY_DO_MIGRATION"

const EnvQueueUrl = "QUEUE_URL"
const EnvQueueClusterId = "QUEUE_CLUSTER_ID"
const EnvQueueClientId = "QUEUE_CLIENT_ID"

const EnvQueueAcknowledgementWaitTime = "QUEUE_ACK_WAIT_TIME"
const EnvQueueMaximumInflightMessages = "QUEUE_MAX_INFLIGHT_MESSAGES"

const EnvProfileServiceUri  = ""

const ConfigQueuesDurableName = "service_notification"
const ConfigQueueMessageInLoggedName = "message_in_logged"
const ConfigQueueMessageInQueuedName = "message_in_queued_%s"
const ConfigQueueMessageOutLoggedName = "message_out_logged"
const ConfigQueueMessageOutChannelledName = "message_out_channel_%s"
