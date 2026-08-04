# Multi-stage Dockerfile for Go microservices.
# Build: docker build --build-arg SERVICE=gateway -t gomessenger/gateway:local .
FROM golang:1.25-alpine AS builder
ARG SERVICE=gateway
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/service ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/service /service
ENTRYPOINT ["/service"]
