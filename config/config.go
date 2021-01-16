package config

const EnvServerPort = "SERVER_PORT"

const EnvDatabaseDriver = "DATABASE_DRIVER"
const EnvDatabaseUrl = "DATABASE_URL"
const EnvReplicaDatabaseUrl = "REPLICA_DATABASE_URL"

const EnvMigrate = "DO_MIGRATION"
const EnvMigrationPath = "MIGRATION_PATH"

const EnvProfileServiceUri  = ""

const EnvQueueMessageInLogged = "QUEUE_MESSAGE_IN_LOGGED"
const EnvQueueMessageInRouteIds = "QUEUE_MESSAGE_IN_ROUTE_IDS"
const EnvQueueMessageOutLogged = "QUEUE_MESSAGE_OUT_LOGGED"
const EnvQueueMessageOutRouteIds = "QUEUE_MESSAGE_OUT_ROUTE_IDS"


const ConfigQueuesDurableName = "service_notification"
const ConfigQueueMessageInLoggedName = "message_in_logged"
const ConfigQueueMessageInRoutedName = "message_in_route_%s"
const ConfigQueueMessageOutLoggedName = "message_out_logged"
const ConfigQueueMessageOutRoutedName = "message_out_route_%s"
