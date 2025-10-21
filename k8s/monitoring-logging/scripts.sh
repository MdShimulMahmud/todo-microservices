#!/bin/bash
# deploy-monitoring.sh
# Deploy complete logging and monitoring stack for Kubernetes

set -e

echo "==========================================="
echo "Deploying Monitoring & Logging Stack"
echo "==========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create namespaces
echo -e "${YELLOW}Creating namespaces...${NC}"
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace logging --dry-run=client -o yaml | kubectl apply -f -

# Add Helm repositories
echo -e "${YELLOW}Adding Helm repositories...${NC}"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

echo -e "${GREEN}✓ Helm repositories added${NC}"

# Deploy kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
echo -e "${YELLOW}Deploying kube-prometheus-stack...${NC}"
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values kube-prometheus-stack-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ kube-prometheus-stack deployed${NC}"

# Deploy Loki
echo -e "${YELLOW}Deploying Loki...${NC}"
helm upgrade --install loki grafana/loki \
  --namespace logging \
  --values loki-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ Loki deployed${NC}"

# Deploy Promtail
echo -e "${YELLOW}Deploying Promtail...${NC}"
helm upgrade --install promtail grafana/promtail \
  --namespace logging \
  --values promtail-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ Promtail deployed${NC}"

# Deploy Tempo
echo -e "${YELLOW}Deploying Tempo...${NC}"
helm upgrade --install tempo grafana/tempo \
  --namespace logging \
  --values tempo-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ Tempo deployed${NC}"

# Apply custom alerts
echo -e "${YELLOW}Applying custom Kubernetes alerts...${NC}"
kubectl apply -f kubernetes-alerts.yaml

echo -e "${GREEN}✓ Custom alerts applied${NC}"

# Apply Grafana dashboard
echo -e "${YELLOW}Applying Grafana dashboard...${NC}"
kubectl apply -f grafana-k8s-dashboard.yaml

echo -e "${GREEN}✓ Grafana dashboard applied${NC}"

# Wait for all pods to be ready
echo -e "${YELLOW}Waiting for all pods to be ready...${NC}"
kubectl wait --for=condition=ready pod --all -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod --all -n logging --timeout=300s

echo -e "${GREEN}✓ All pods are ready${NC}"

# Get Grafana password
echo ""
echo "==========================================="
echo -e "${GREEN}Deployment Complete!${NC}"
echo "==========================================="
echo ""
echo "Access Information:"
echo "-------------------"
echo ""
echo "Grafana:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80"
echo "  URL: http://localhost:3000"
echo "  Username: admin"
echo "  Password: admin (change in production!)"
echo ""
echo "Prometheus:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090"
echo "  URL: http://localhost:9090"
echo ""`
echo "Alertmanager:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093"
echo "  URL: http://localhost:9093"
echo ""
echo "Loki:"
echo "  Port-forward: kubectl port-forward -n logging svc/loki-gateway 3100:80"
echo "  URL: http://localhost:3100"
echo ""
echo "Tempo:"
echo "  Port-forward: kubectl port-forward -n logging svc/tempo-gateway 3200:80"
echo "  URL: http://localhost:3200"
echo ""
echo "Pre-configured datasources in Grafana:"
echo "  - Prometheus (metrics)"
echo "  - Loki (logs)"
echo "  - Tempo (traces)"
echo ""
echo "Log Retention: 2 days (48 hours)"
echo "Trace Retention: 2 days (48 hours)"
echo "Metrics Retention: 15 days"
echo ""
echo "==========================================="