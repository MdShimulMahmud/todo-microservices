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
kubectl create namespace tracing --dry-run=client -o yaml | kubectl apply -f - # Ensure tracing ns exists

# Add Helm repositories
echo -e "${YELLOW}Adding Helm repositories...${NC}"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

echo -e "${GREEN}✓ Helm repositories added${NC}"

# Deploy kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
echo -e "${YELLOW}Deploying kube-prometheus-stack...${NC}"
helm upgrade --install prom-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values kube-prometheus-stack-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ prom-stack deployed${NC}"

# Deploy Loki
echo -e "${YELLOW}Deploying Loki...${NC}"
helm upgrade --install loki grafana/loki-stack \
  --namespace logging \
  --values loki-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ Loki deployed${NC}"


# Deploy Tempo
echo -e "${YELLOW}Deploying Tempo...${NC}"
# Use upgrade --install and add --wait
helm upgrade --install tempo grafana/tempo \
  --namespace tracing \
  --values tempo-values.yaml \
  --wait \
  --timeout 10m

echo -e "${GREEN}✓ Tempo deployed${NC}"

# Apply custom alerts
echo -e "${YELLOW}Applying custom Kubernetes alerts...${NC}"
# Ensure kubernetes-alerts.yaml exists and contains valid PrometheusRule resources
if [ -f "kubernetes-alerts.yaml" ]; then
  kubectl apply -f kubernetes-alerts.yaml # Apply alerts to monitoring ns
  echo -e "${GREEN}✓ Custom alerts applied${NC}"
else
  echo -e "${YELLOW}Skipping custom alerts: kubernetes-alerts.yaml not found${NC}"
fi


# Apply service monitors
echo -e "${YELLOW}Applying Service Monitor...${NC}"
# Ensure service-monitor.yaml exists and contains valid ServiceMonitor resources
if [ -f "service-monitor.yaml" ]; then
  kubectl apply -f service-monitor.yaml
  echo -e "${GREEN}✓ Service Monitor applied${NC}"
else
  echo -e "${YELLOW}Skipping Service Monitor: service-monitor.yaml not found${NC}"
fi

# Wait for all pods to be ready
echo -e "${YELLOW}Waiting for all pods to be ready...${NC}"
kubectl wait --for=condition=ready pod --all -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod --all -n logging --timeout=300s
kubectl wait --for=condition=ready pod --all -n tracing --timeout=300s # Wait for Tempo pods too

echo -e "${GREEN}✓ All pods are ready${NC}"

# Get Grafana password (or provide instructions)
GRAFANA_SECRET_NAME=$(kubectl get secret -n monitoring -l app.kubernetes.io/name=grafana -o jsonpath='{.items[0].metadata.name}')
GRAFANA_PASSWORD=$(kubectl get secret -n monitoring ${GRAFANA_SECRET_NAME} -o jsonpath="{.data.admin-password}" | base64 --decode)

echo ""
echo "==========================================="
echo -e "${GREEN}Deployment Complete!${NC}"
echo "==========================================="
echo ""
echo "Access Information:"
echo "-------------------"
echo ""
echo "Grafana:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/prom-stack-grafana 3000:80"
echo "  URL: http://localhost:3000"
echo "  Username: admin"
# Update password info
echo "  Password: ${GRAFANA_PASSWORD}"
echo ""
echo "Prometheus:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/prom-stack-prometheus 9090:9090"
echo "  URL: http://localhost:9090"
echo ""
echo "Alertmanager:"
echo "  Port-forward: kubectl port-forward -n monitoring svc/prom-stack-alertmanager 9093:9093"
echo "  URL: http://localhost:9093"
echo ""
echo "Loki:"
# Verify service name, loki-stack-gateway is common if release name is loki-stack
echo "  Port-forward: kubectl port-forward -n logging svc/loki-stack-gateway 3100:80"
echo "  URL: http://localhost:3100 (For Grafana Datasource: http://loki-stack-gateway.logging.svc.cluster.local:80 or check svc)"
echo ""
echo "Tempo:"
# Update namespace and likely service name/port
echo "  Port-forward: kubectl port-forward -n tracing svc/tempo 3200:3200" # Use correct service and port 3200
echo "  URL: http://localhost:3200 (For Grafana Datasource: http://tempo.tracing.svc.cluster.local:3200)"
echo ""
# Update datasource info
echo "Data Sources in Grafana:"
echo "  - Prometheus (metrics) - Should be auto-configured by the chart."
echo "  - Loki (logs) - *Needs manual configuration* (Use URL above)"
echo "  - Tempo (traces) - *Needs manual configuration* (Use URL above)"
echo ""
echo "Log Retention: 2 days (48 hours)"
echo "Trace Retention: 2 days (48 hours)"
# Metrics retention depends on Prometheus config, often defaults to 15d
echo "Metrics Retention: (Check Prometheus config, often 15 days)"
echo ""
echo "==========================================="