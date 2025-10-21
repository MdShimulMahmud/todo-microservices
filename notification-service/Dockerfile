FROM golang:1.24-alpine AS builder
WORKDIR /src

# Copy module files and proto so replace works
COPY ./notification-service/go.mod ./notification-service/go.sum ./
COPY ./proto /proto
RUN go mod download

# Copy source and build
COPY ./notification-service .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/notification-service

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/notification-service /notification-service
USER nonroot:nonroot

ARG PORT=50053
ENV PORT=${PORT}
# MONGO_URI will be provided via Kubernetes Secret at runtime.

EXPOSE ${PORT}
ENTRYPOINT ["/notification-service"]