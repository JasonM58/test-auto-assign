# =========================
# Stage 1: Build binary
# =========================
FROM golang:1.25.1-alpine AS build-env

ENV APP_PATH="/app"
ENV GOPRIVATE="github.com/ionextai/*"

# Create non-root user
RUN adduser -D -u 1000 -g '' appuser

# Install tzdata & certs
RUN apk add --no-cache tzdata ca-certificates

WORKDIR ${APP_PATH}

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o main ./cmd/main.go
RUN chmod +x /app/main && chown appuser:appuser /app/main

# =========================
# Stage 2: Runtime
# =========================
FROM scratch

# Copy timezone data, certs, and passwd
COPY --from=build-env /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build-env /etc/passwd /etc/passwd
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Switch to non-root
USER appuser
WORKDIR /app

# Copy app binary
COPY --from=build-env /app/main .

# Set timezone
ENV TZ=Asia/Jakarta

CMD ["./main"]
