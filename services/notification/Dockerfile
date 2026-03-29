# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o notification-service ./services/notification/cmd

# Run stage
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/notification-service .
COPY --from=builder /app/frontend ./frontend
EXPOSE 8080
CMD ["./notification-service"]
