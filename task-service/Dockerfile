FROM golang:1.24-alpine AS builder

WORKDIR /src

# Copy module files for the service and the proto module so local replace directives work
COPY ./task-service/go.mod ./task-service/go.sum ./
COPY ./proto /proto

# Download dependencies
RUN go mod download

# Copy the service source and build
COPY ./task-service .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/task-service

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/task-service /task-service
USER nonroot:nonroot

ARG PORT=50051
ENV PORT=${PORT}
# Do NOT bake MONGO_URI into the image. Kubernetes should provide runtime
# configuration via env or secrets. Any DB/service addresses must be injected
# by the deployment (via Secret or ConfigMap).

EXPOSE ${PORT}
ENTRYPOINT ["/task-service"]