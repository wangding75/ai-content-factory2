FROM golang:1.26-alpine AS build
WORKDIR /src

# Copy go.mod and go.sum for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22

# Install curl to ensure the healthcheck tool exists in the final image
RUN apk add --no-cache curl

# Create non-root user
RUN adduser -D -H -u 10001 appuser

# Set working directory to /app
WORKDIR /app

# Copy binaries and migrations folder
COPY --from=build /out/api ./api
COPY --from=build /out/migrate ./migrate
COPY migrations ./migrations

# Adjust ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

EXPOSE 8080

# Default command to run Go API
CMD ["./api"]