FROM golang:1.24-alpine AS builder
WORKDIR /src

# Copy module files and proto so replace works
COPY ./analytics-service/go.mod ./analytics-service/go.sum ./
COPY ./proto /proto
RUN go mod download

# Copy source and build
COPY ./analytics-service .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/analytics-service

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/analytics-service /analytics-service
USER nonroot:nonroot

ARG PORT=50054
ENV PORT=${PORT}
# Keep DB/service addresses dynamic and provided by deployment environment.

EXPOSE ${PORT}
ENTRYPOINT ["/analytics-service"]