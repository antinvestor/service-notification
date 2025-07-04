FROM golang:1.24 as builder

WORKDIR /app

# Copy go.mod and go.sum files from the project root
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary from the cmd directory
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/notification_binary ./apps/default/cmd/

FROM gcr.io/distroless/static:nonroot

USER 65532:65532
EXPOSE 80
EXPOSE 50051

WORKDIR /

COPY --from=builder /app/notification_binary /notification
COPY --from=builder /app/migrations /migrations

# Run the service command by default when the container starts.
ENTRYPOINT ["/notification"]
