service-notification
A simple repository with the bare bone things that will always be useful for any notification service being developed for ant investor especially if it relies on golang

Need to start Nats streaming server
nats-streaming-server --store file --dir ./data --max_msgs 0 --max_btyes 0

run server
go run main.go

run client server
go run main.go client