ARG TARGETOS=linux
ARG TARGETARCH=amd64

# ---------- Builder ----------
FROM golang:1.25 AS builder

# Add Maintainer Info
LABEL maintainer="Bwire Peter <bwire517@gmail.com>"

WORKDIR /app

ARG VERSION=dev
ARG REVISION=none
ARG BUILDTIME

# Copy go.mod and go.sum files from the project root
COPY go.mod go.sum ./
RUN go mod download

# Copy project files
COPY ./apps/default ./apps/default
COPY ./internal ./internal

# Build static binary for target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
     -ldflags="-s -w \
         -X github.com/pitabwire/frame.Version=${VERSION} \
         -X github.com/pitabwire/frame.Commit=${REVISION} \
         -X github.com/pitabwire/frame.Date=${BUILDTIME}" \
     -o /app/binary ./apps/default/cmd/main.go

# ---------- Final ----------
FROM cgr.dev/chainguard/static:latest

USER 65532:65532

EXPOSE 80

# Add OCI metadata labels
ARG VERSION
ARG REVISION
ARG BUILDTIME
LABEL org.opencontainers.image.title="Notification Service"
LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.revision=$REVISION
LABEL org.opencontainers.image.created=$BUILDTIME
LABEL org.opencontainers.image.source="https://github.com/antinvestor/service-notification"

WORKDIR /

COPY --from=builder /app/binary /notification
COPY --from=builder /app/apps/default/migrations /migrations

# Run the service command by default when the container starts.
ENTRYPOINT ["/notification"]
