FROM golang:1.24-alpine AS builder
WORKDIR /src

# Copy module files and proto so replace works
COPY ./user-service/go.mod ./user-service/go.sum ./
COPY ./proto /proto
RUN go mod download

# Copy source and build
COPY ./user-service .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/user-service

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/user-service /user-service
USER nonroot:nonroot

ARG PORT=50052
ENV PORT=${PORT}
# MONGO_URI must be injected at runtime via Kubernetes Secret/ConfigMap.

EXPOSE ${PORT}
ENTRYPOINT ["/user-service"]