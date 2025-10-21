FROM golang:1.24-alpine AS builder
WORKDIR /src

# Copy module files and proto for local replace
COPY ./api-gateway/go.mod ./api-gateway/go.sum ./
COPY ./proto /proto
RUN go mod download

# Copy source and build
COPY ./api-gateway .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/api-gateway

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/api-gateway /api-gateway
USER nonroot:nonroot

ARG PORT=8080
ENV PORT=${PORT}
# Do not bake backend addresses into the image. Provide at runtime via
# ConfigMap or environment variables in the Kubernetes Deployment so addresses
# can change per environment.

# Expose the port
EXPOSE ${PORT}

# Run the application (absolute path recommended for distroless)
ENTRYPOINT ["/api-gateway"]