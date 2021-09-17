package config

const EnvServerPort = "PORT"

const EnvDatabaseUrl = "DATABASE_URL"
const EnvReplicaDatabaseUrl = "REPLICA_DATABASE_URL"

const EnvMigrate = "DO_MIGRATION"
const EnvMigrationPath = "MIGRATION_PATH"

const EnvOauth2JwtVerifyAudience = "OAUTH2_JWT_VERIFY_AUDIENCE"
const EnvOauth2JwtVerifyIssuer = "OAUTH2_JWT_VERIFY_ISSUER"

const EnvProfileServiceUri = "PROFILE_SERVICE_URI"

const EnvQueueMessageInLogged = "QUEUE_MESSAGE_IN_LOGGED"
const EnvQueueMessageInRouteIds = "QUEUE_MESSAGE_IN_ROUTE_IDS"
const EnvQueueMessageOutLogged = "QUEUE_MESSAGE_OUT_LOGGED"
const EnvQueueMessageOutRouteIds = "QUEUE_MESSAGE_OUT_ROUTE_IDS"

const QueuesDurableName = "service_notification"
const QueueMessageInLoggedName = "message_in_logged"
const QueueMessageInRoutedName = "message_in_route_%s"
const QueueMessageOutLoggedName = "message_out_logged"
const QueueMessageOutRoutedName = "message_out_route_%s"
