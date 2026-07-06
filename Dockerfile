FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o sprig-db ./cmd/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/sprig-db .
# Copy static files and templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# Expose port
EXPOSE 7777

# Set production environment defaults
ENV SPRIG_JWT_SECRET="PLEASE_CHANGE_THIS_IN_PRODUCTION"

# Run the binary
CMD ["./sprig-db"]
