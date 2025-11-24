FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app .
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/app .
RUN chown appuser:appgroup /app/app
USER appuser
CMD ["./app"]
