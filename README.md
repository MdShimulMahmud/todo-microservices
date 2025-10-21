# TODO Application — Tasks & Implementation Guide (Q-1)

This README implements Q-1: step-by-step guidance to provision a Kubernetes cluster (1 control-plane + 2 workers), build optimized Docker images for a 5-service TODO app, deploy the app and MongoDB with PV/PVC and constraints, and add logging/APM and monitoring with retention and alerts.

This guide uses kind and Helm for reproducibility. Replace with cloud provider tools (eksctl/gcloud/aksctl/kops) as needed.

---

## Tasks overview

- Task 1: Provision a Kubernetes cluster and create PV/PVC; view CPU & memory metrics in Grafana.
- Task 2: Create, optimize, and push Docker images for the 5 microservices (gRPC + REST endpoints).
- Task 3: Deploy the application in Kubernetes meeting constraints (2 nodes, MongoDB StatefulSet, Secrets, taint/toleration, Envoy sidecar, resource requests/limits, fixed scaling, 2Gi storage)
- Task 4: Logging & APM (Grafana Loki + Tempo), 2 day retention, and monitoring (Prometheus + Grafana + alerts)

---

## Task 1 — Provision a Kubernetes cluster (kind example)

1) Install Docker, kind, kubectl, and helm.

2) Create a kind config `kind-cluster.yaml` for 1 control-plane and 2 workers:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
```

Create the cluster:

```bash
kind create cluster --config kind-cluster.yaml --name todo-cluster
kubectl cluster-info --context kind-todo-cluster
```
You can provision your cluster on any cloud provider as needed or use virtual machines with kubeadm.

Follow the instructions described in here : [https://github.com/MdShimulMahmud/ansible-k8s](https://github.com/MdShimulMahmud/ansible-k8s)

1) Provision storage (local-path provisioner recommended for kind):

```bash
helm repo add rancher-local-path https://rancher.github.io/local-path-provisioner
helm repo update
helm upgrade --install local-path-storage rancher-local-path/local-path-provisioner --namespace local-path-storage --create-namespace
kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

4) Verify nodes and storage:

```bash
kubectl get nodes
kubectl get sc
```

---

## Task 2 — Optimize Docker images and push to Docker Hub

Use a multi-stage build and distroless runtime to minimize image size.

Example Dockerfile (Go service):

```dockerfile
# stage: builder
FROM golang:1.20-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/app ./cmd/app

# stage: runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
```

Build and push (Repeat for all services):

```bash
# build
docker build -t shimulmahmud/task-service:1.0.0 -f task-service/Dockerfile ./task-service
# push
docker push shimulmahmud/task-service:1.0.0
```

Repeat for all 5 services: task-service, user-service, notification-service, analytics-service, api-gateway.

Notes:
- Use `-ldflags "-s -w"` to strip symbol information.
- Use non-root runtime images (distroless/scratch) to reduce attack surface.
- Make repositories public on Docker Hub so they are pullable without credentials.

---

## Task 3 — Kubernetes manifests and deployment

The following examples implement the constraints you requested. Add them to `k8s/` or adjust existing manifests.

1) Namespace and ServiceAccount

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: todo-app
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: todo-app-sa
  namespace: todo-app
```

2) Secrets: MongoDB credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mongodb-secret
  namespace: todo-app
type: Opaque
stringData:
  username: admin # use base64 encoding in production
  password: password123 # use base64 encoding in production
```

3) PVC for application storage (2Gi)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-storage-pvc
  namespace: todo-app
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
  storageClassName: standard
```

4) MongoDB StatefulSet + headless Service (2Gi per replica)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mongodb
  namespace: todo-app
spec:
  clusterIP: None
  selector:
    app: mongodb
  ports:
    - port: 27017
+      name: mongodb
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mongodb
  namespace: todo-app
spec:
  serviceName: mongodb
  replicas: 1
  selector:
    matchLabels:
      app: mongodb
  template:
    metadata:
      labels:
        app: mongodb
    spec:
      containers:
        - name: mongodb
          image: mongo:5.0
          ports:
            - containerPort: 27017
              name: mongodb
          env:
            - name: MONGO_INITDB_ROOT_USERNAME
              valueFrom:
                secretKeyRef:
                  name: mongodb-secret
                  key: username
            - name: MONGO_INITDB_ROOT_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mongodb-secret
                  key: password
          volumeMounts:
            - name: mongodb-data
              mountPath: /data/db
  volumeClaimTemplates:
    - metadata:
        name: mongodb-data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 2Gi
        storageClassName: standard
```

5) Taint nodes (example):

```bash
# mark two nodes with a taint so only tolerating pods can run there
kubectl taint nodes <node1> app=todo:NoSchedule
kubectl taint nodes <node2> app=todo:NoSchedule
```

6) App Deployment (task-service) — includes Envoy sidecar, toleration, podAntiAffinity, and resource requests/limits

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: task-service
  namespace: todo-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: task-service
  template:
    metadata:
      labels:
        app: task-service
    spec:
      serviceAccountName: todo-app-sa
      tolerations:
        - key: "app"
          operator: "Equal"
          value: "todo"
          effect: "NoSchedule"
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: task-service
              topologyKey: "kubernetes.io/hostname"
      containers:
        - name: task-service
          image: shimulmahmud/task-service:1.0.0
          ports:
            - containerPort: 50051
              name: grpc
          env:
            - name: MONGO_HOST
              value: "mongodb-0.mongodb.todo-app.svc.cluster.local:27017"
            - name: MONGO_USERNAME
              valueFrom:
                secretKeyRef:
                  name: mongodb-secret
                  key: username
            - name: MONGO_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mongodb-secret
                  key: password
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
        - name: envoy
          image: envoyproxy/envoy:v1.22.0
          ports:
            - containerPort: 9901
              name: admin
            - containerPort: 8080
              name: http
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "100m"
              memory: "128Mi"
```

7) Service for task-service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: task-service
  namespace: todo-app
spec:
  selector:
    app: task-service
  ports:
    - name: grpc
      port: 50051
      targetPort: 50051
  type: ClusterIP
```

8) Fixed scaling is implemented by `replicas: 2` in each Deployment. Pod anti-affinity forces scheduling across nodes where possible.

---

## Task 4 — Logging, APM, retention and monitoring

1) Install observability stack (script in repo `k8s/monitoring-logging/scripts.sh`) or manually via Helm:

```bash
# from repo
cd k8s/monitoring-logging
chmod +x scripts.sh
./scripts.sh
```

This installs kube-prometheus-stack, Loki, Promtail, and Tempo with the repo values files.

2) Loki & Tempo retention (set to 48h)

Example Loki values snippet (values.yaml):

```yaml
loki:
  config:
    table_manager:
      retention_deletes_enabled: true
    limits_config:
      retention_period: 48h
persistence:
  enabled: true
  size: 10Gi
```

Tempo values snippet:

```yaml
compactor:
  retention: 48h
persistence:
  enabled: true
  size: 10Gi
```

3) Reduce storage usage

- Sample logs at the application or promtail level.
- Reduce log verbosity for production (e.g. avoid debug in high-throughput components).
- Set alerts for PVC usage to prevent full disks.

4) Prometheus & Grafana alerts: example PrometheusRule for high memory usage

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: node-alerts
  namespace: monitoring
spec:
  groups:
  - name: node.rules
+    rules:
+    - alert: NodeMemoryHigh
+      expr: (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes > 0.85
+      for: 5m
+      labels:
+        severity: warning
+      annotations:
+        summary: "Node memory usage > 85%"
+        description: "Node memory used is >85% for more than 5 minutes"
```

5) Visualize metrics in Grafana

- Port-forward Grafana and open http://localhost:3000

```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
```

---

## Verification checklist

- `kubectl get nodes`
- `kubectl get pods -n todo-app`
- `kubectl get pvc -n todo-app`
- Grafana: http://localhost:3000 (port-forward)
- Loki/Tempo: port-forward and query logs/traces

