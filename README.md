service-notification

A repository for the  notification service being developed 
for ant investor

### How do I get set up? ###

* The api definition is found at bitbucket.org/antinvestor/api/service
* To update the proto service you need to run the command :
    `protoc -I ../api/service/profile/v1/ ../api/service/profile/v1/profile.proto --go_out=plugins=grpc:grpc/profile`
    `protoc -I ../api/service/health/v1/ ../api/service/health/v1/health.proto --go_out=plugins=grpc:grpc/health`
    `protoc -I ../api/service/notification/v1/ ../api/service/notification/v1/notification.proto --go_out=plugins=grpc:grpc/notification`

    with that in place update the implementation appropriately considering the profile project bare bones.


Need to start Nats streaming server
nats-streaming-server --store file --dir ./data --max_msgs 0 --max_btyes 0

run server
go run main.go

run client server
go run main.go client