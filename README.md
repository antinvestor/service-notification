service-notification

A repository for the  notification service being developed 
for ant investor

### How do I get set up? ###

Need to start Nats streaming server
nats-streaming-server --store file --dir ./data --max_msgs 0 --max_btyes 0

run server
go run main.go

run client server
go run main.go client