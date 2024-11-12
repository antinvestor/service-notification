FROM golang:1.23 as builder

WORKDIR /

COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the local package files to the container's workspace.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o notification_binary .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /notification_binary /notification
COPY --from=builder /migrations /migrations

WORKDIR /

# Run the service command by default when the container starts.
ENTRYPOINT ["/notification"]

# Document the port that the service listens on by default.
EXPOSE 7020
