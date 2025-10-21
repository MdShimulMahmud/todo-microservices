# TO-DO Application with Microservices

A scalable, cloud-native TO-DO application built with microservices architecture using gRPC for internal communication and REST APIs for external clients.

## Architecture Overview

# TO-DO Application with Microservices

This repository contains a small microservices-based TO-DO application implemented in Go. Services communicate internally via gRPC and expose an HTTP API through an API Gateway. The project includes Kubernetes manifests (in `k8s/`) to deploy the application, MongoDB, and a basic observability stack (Prometheus, Grafana, Loki, Tempo).

---

# Quick status — compatibility

- Dockerfiles in each service use multi-stage Go builds and produce small, non-root runtime images (distroless/static:nonroot). They expose the ports used by the manifests and provide runtime-config via environment variables. Images no longer bake service addresses or database URIs — those are provided by Kubernetes at deploy time via Secrets and ConfigMaps.

Important: The k8s manifests in `k8s/` have been updated to reference images under your Docker Hub username `shimulmahmud` (for the microservices). Before deploying, build and push the images below (or edit the manifests to point to your registry of choice):

Services and expected image names:
- shimulmahmud/api-gateway:latest
- shimulmahmud/task-service:latest
- shimulmahmud/user-service:latest
- shimulmahmud/notification-service:latest
- shimulmahmud/analytics-service:latest

---

## Architecture overview

- Task Service (gRPC) — port 50051
- User Service (gRPC) — port 50052
- Notification Service (gRPC) — port 50053
- Analytics Service (gRPC) — port 50054
- API Gateway (HTTP) — port 8080
- MongoDB (StatefulSet + PVC)
- Envoy: configured as a sidecar in `task-service` for service-to-service communication
- Observability: Prometheus, Grafana, Loki (logs), Tempo (traces)

---

## Prerequisites

- Docker (to build images)
- Kubernetes cluster (minikube / kind / a cloud cluster)
- kubectl configured to target the cluster
- If images are pushed to a private registry, you need credentials (imagePullSecrets)

---

## Build & publish images (PowerShell examples)

Build locally and push to Docker Hub (images expected by the manifests are listed above). Replace `shimulmahmud` with your registry prefix if different.

```pwsh
# Build
docker build -t shimulmahmud/task-service:latest -f task-service/Dockerfile ./
docker build -t shimulmahmud/user-service:latest -f user-service/Dockerfile ./
docker build -t shimulmahmud/notification-service:latest -f notification-service/Dockerfile ./
docker build -t shimulmahmud/analytics-service:latest -f analytics-service/Dockerfile ./
docker build -t shimulmahmud/api-gateway:latest -f api-gateway/Dockerfile ./

# Push (after docker login)
docker push shimulmahmud/task-service:latest
docker push shimulmahmud/user-service:latest
docker push shimulmahmud/notification-service:latest
docker push shimulmahmud/analytics-service:latest
docker push shimulmahmud/api-gateway:latest
```

Notes:
- Manifests in `k8s/` now reference images under `shimulmahmud/`. If you want a different registry, update the image fields accordingly.
- `imagePullPolicy: Always` is used for `:latest` images to force fresh pulls in clusters.

---

## Deploy to Kubernetes (step-by-step)

1) Create the namespace:

```pwsh
kubectl apply -f k8s/namespace.yaml
```

2) Create MongoDB secrets and storage, then deploy MongoDB:

We store a pre-built MongoDB connection URI in `k8s/mongodb-secret.yaml` under the `uri` key (base64). If you need to regenerate the base64-encoded URI locally (Windows PowerShell), run:

```pwsh
$username = 'admin'       # replace if different
$password = 'password123' # replace with your password
$uri = "mongodb://$username:$password@mongodb:27017/todo_app?authSource=admin"
[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($uri))
```

Copy the resulting base64 string into `k8s/mongodb-secret.yaml` under `data.uri` (or update the file programmatically). Then apply:

```pwsh
kubectl apply -f k8s/mongodb-secret.yaml
kubectl apply -f k8s/mongodb-pv-pvc.yaml
kubectl apply -f k8s/mongodb-deployment.yaml
```

3) Ensure nodes and taints (one-time cluster prep)

The manifests assume pods will run on two nodes (node1, node2) and use a taint/key `app=todo:NoSchedule`. Label and taint nodes accordingly if you want to restrict scheduling:

```pwsh
# label nodes (run on control plane)
kubectl label node <node1> kubernetes.io/hostname=node1
kubectl label node <node2> kubernetes.io/hostname=node2

# add taint to nodes you want to reserve
kubectl taint nodes <node1> app=todo:NoSchedule
kubectl taint nodes <node2> app=todo:NoSchedule
```

The pods include tolerations so they can still be scheduled onto tainted nodes.

4) Deploy Envoy config (ConfigMap) and microservices

This repo includes a small `app-config` ConfigMap (`k8s/app-config.yaml`) containing backend addresses which the `api-gateway` Deployment consumes. The app deployments read `MONGO_URI` directly from `k8s/mongodb-secret.yaml` (`data.uri`).

```pwsh
kubectl apply -f k8s/envoy-configmap.yaml
kubectl apply -f k8s/app-config.yaml
kubectl apply -f k8s/task-service.yaml
kubectl apply -f k8s/user-service.yaml
kubectl apply -f k8s/notification-service.yaml
kubectl apply -f k8s/analytics-service.yaml
kubectl apply -f k8s/api-gateway.yaml
```

5) Deploy observability stack

```pwsh
kubectl apply -f k8s/monitoring/grafana-prometheus.yaml
kubectl apply -f k8s/monitoring/grafana-loki.yaml
kubectl apply -f k8s/monitoring/grafana-tempo.yaml
kubectl apply -f k8s/monitoring/prometheus-rules.yaml   # optional; requires Prometheus Operator
```

6) Verify basic health

```pwsh
kubectl get pods -n todo-app
kubectl get pvc -n todo-app
kubectl get svc -n todo-app
```

7) Smoke test the API gateway

If your API Gateway Service is a LoadBalancer or NodePort, call the health endpoint:

```pwsh
# if LoadBalancer
curl http://<external-ip-or-dns>/health
# or if nodeport (example):
kubectl get svc -n todo-app api-gateway -o yaml
curl http://<node-ip>:<nodeport>/health
```

---

## Logs, Traces and Retention

- Loki is configured to retain logs for 48 hours (2 days). The config lives in `k8s/monitoring/grafana-loki.yaml` and stores data to `loki-pvc`.
- Tempo compactor is configured to retain traces for 48 hours.
- Make sure PVCs used by Loki and Tempo are large enough for your expected ingestion to avoid data loss.

To inspect logs via Grafana (Loki datasource):

1. Open Grafana UI (NodePort/LoadBalancer) and go to Explore → Loki
2. Run a query such as `{app="task-service"}`

---

## Monitoring & Alerts

- Prometheus scrapes pods and nodes (configured in `k8s/monitoring/grafana-prometheus.yaml`).
- A small set of alert rules (NodeNotReady, HighCPUUsage, PodCrashLooping) is included under `k8s/monitoring/prometheus-rules.yaml`. These require the Prometheus Operator CRDs to be installed.
- Grafana is provisioned with datasources for Prometheus, Loki, and Tempo in the manifests.

---

## Dockerfile compatibility notes

- All service Dockerfiles use `CGO_ENABLED=0 GOOS=linux` and build static binaries suitable for distroless images.
- Runtime images use `gcr.io/distroless/static:nonroot` and switch to a non-root user; this matches the pod securityContext `runAsNonRoot: true` in the manifests.
- Each Dockerfile exposes the ports the corresponding manifest expects and provides default `PORT`/`MONGO_URI` environment variables that manifests override from secrets when deployed.
- Recommendation (optional): change `api-gateway/Dockerfile` to use `CMD ["/api-gateway"]` instead of `CMD ["./api-gateway"]` to be explicit about the binary path.

---

## Troubleshooting

- If pods are Pending: check node labels and taints, and ensure PVC is bound.
- If images fail to pull: confirm image names/tags, registry accessibility, or add `imagePullSecrets`.
- If Envoy/gRPC traffic fails: ensure Envoy sidecar is present in the same pod and the Envoy config matches the service ports.

---

## Next steps / optional improvements

- Replace `:latest` tags with fixed semantic versions for reproducible deployments.
- Add PodDisruptionBudgets and (optionally) HPAs for better availability.
- Add more Prometheus alert rules and Grafana dashboards tailored to your SLAs.

If you want, I can:
- Update the `api-gateway/Dockerfile` to use the absolute binary path.
- Deploy the manifests to a local `kind` cluster and run an end-to-end smoke test.

---

Happy to update anything else — tell me which next step you want me to take.
