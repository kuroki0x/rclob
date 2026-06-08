#!/usr/bin/env bash
# deploy.sh - One-command deployment for rclob on Kubernetes
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

REGISTRY="${REGISTRY:-registry.example.com}"
IMAGE_TAG="${IMAGE_TAG:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
ENV="${ENV:-staging}"
NAMESPACE="${NAMESPACE:-rclob}"

echo "=========================================="
echo " rclob Kubernetes Deployment"
echo "=========================================="
echo "  Registry:   $REGISTRY"
echo "  Image Tag:  $IMAGE_TAG"
echo "  Environment: $ENV"
echo "  Namespace:   $NAMESPACE"
echo "=========================================="

# Step 1: Build and push image
echo ""
echo ">> Step 1: Building Docker image..."
make k8s-image

echo ""
echo ">> Step 2: Pushing image to registry..."
make k8s-push

# Step 2: Apply K8s manifests
echo ""
echo ">> Step 3: Creating namespace..."
make k8s-apply-ns

echo ""
echo ">> Step 4: Applying K8s manifests..."
make k8s-apply

# Step 3: Wait for rollout
echo ""
echo ">> Step 5: Waiting for rollout..."
make k8s-rollout-status

echo ""
echo "=========================================="
echo " Deployment complete!"
echo "=========================================="
echo "  Service:    $NAMESPACE/$APP_NAME"
echo "  Health:     http://localhost:8080/health"
echo "  Logs:       make k8s-logs"
echo "  Port:       make k8s-port-forward"
echo "=========================================="
