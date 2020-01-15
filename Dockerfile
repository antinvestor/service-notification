FROM golang:1.13 as builder

WORKDIR /go/src/antinvestor.com/service/notification

ADD go.mod ./
RUN go mod download

# Copy the local package files to the container's workspace.
ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o notification_binary .

FROM scratch
COPY --from=builder /go/src/antinvestor.com/service/notification/notification_binary /notification
COPY --from=builder /go/src/antinvestor.com/service/notification/migrations /migrations
#WORKDIR /

# Run the service command by default when the container starts.
ENTRYPOINT ["/notification"]

# Document the port that the service listens on by default.
EXPOSE 7020
