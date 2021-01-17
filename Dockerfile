FROM golang:1.14 as builder

WORKDIR /
# Copy the local package files to the container's workspace.
ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o notification_binary .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /notification_binary /notification
COPY --from=builder /migrations /migrations

WORKDIR /

# Run the service command by default when the container starts.
ENTRYPOINT ["/notification"]

# Document the port that the service listens on by default.
EXPOSE 7020
