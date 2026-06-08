APP_NAME := rclob
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BINARY := $(APP_NAME)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
REGISTRY ?= registry.example.com
NAMESPACE ?= rclob
ENV ?= staging

# Go build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# K8s defaults
KUBE_CONTEXT ?= default
KUBE_NAMESPACE ?= rclob
IMAGE_TAG ?= $(VERSION)

.PHONY: all build clean test run docker docker-up help

all: build

build:
	@echo "Building $(APP_NAME) $(VERSION) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 go build \
		$(LDFLAGS) \
		-o bin/$(BINARY) \
		./cmd/server/
	@echo "Build complete: bin/$(BINARY)"

build-linux:
	@echo "Building $(APP_NAME) for linux/amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build \
		$(LDFLAGS) \
		-o bin/$(BINARY)-linux-amd64 \
		./cmd/server/

build-darwin:
	@echo "Building $(APP_NAME) for darwin/amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build \
		$(LDFLAGS) \
		-o bin/$(BINARY)-darwin-amd64 \
		./cmd/server/

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	@echo "Clean complete"

test:
	@echo "Running tests..."
	go test -v -race -cover ./...

test-short:
	@echo "Running short tests..."
	go test -v -short ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

run:
	@echo "Starting $(APP_NAME)..."
	go run ./cmd/server/

docker:
	@echo "Building Docker image $(APP_NAME):$(VERSION)..."
	docker build -t $(APP_NAME):$(VERSION) .
	@echo "Docker image built: $(APP_NAME):$(VERSION)"

docker-up:
	@echo "Starting $(APP_NAME) in Docker..."
	docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-e REDIS_ADDR=redis:6379 \
		-e PORT=8080 \
		--link redis:redis \
		$(APP_NAME):$(VERSION)

docker-compose-up:
	@echo "Starting service with Docker Compose..."
	docker-compose up -d

docker-compose-down:
	@echo "Stopping service with Docker Compose..."
	docker-compose down

lint:
	@echo "Running linters..."
	go vet ./...
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

install:
	@echo "Installing $(APP_NAME)..."
	go install ./cmd/server/

k8s-build: k8s-image
	@echo "K8s image ready: $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)"

k8s-image:
	@echo "Building Docker image $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)..."
	docker build \
		--build-arg VERSION=$(IMAGE_TAG) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG) \
		-t $(REGISTRY)/$(APP_NAME):latest \
		.
	@echo "Image tagged: $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)"

k8s-push:
	@echo "Pushing image $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)..."
	docker push $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(APP_NAME):latest
	@echo "Image pushed"

k8s-apply:
	@echo "Applying K8s manifests for $(ENV) environment..."
	kustomize build deploy/k8s/overlays/$(ENV) | kubectl apply -f -
	@echo "Applied for environment: $(ENV)"

k8s-apply-ns:
	@echo "Creating namespace..."
	kubectl apply -f deploy/k8s/namespace.yaml
	@echo "Namespace created"

k8s-delete:
	@echo "Deleting K8s resources for $(ENV) environment..."
	kustomize build deploy/k8s/overlays/$(ENV) | kubectl delete -f - --ignore-not-found
	@echo "Deleted for environment: $(ENV)"

k8s-delete-ns:
	@echo "Deleting namespace..."
	kubectl delete namespace $(NAMESPACE) --ignore-not-found
	@echo "Namespace deleted"

k8s-rollout-status:
	@echo "Checking rollout status..."
	kubectl rollout status deployment/$(APP_NAME) -n $(KUBE_NAMESPACE) --timeout=120s

k8s-restart:
	@echo "Restarting deployment..."
	kubectl rollout restart deployment/$(APP_NAME) -n $(KUBE_NAMESPACE)

k8s-logs:
	@echo "Following logs..."
	kubectl logs -l app=$(APP_NAME) -n $(KUBE_NAMESPACE) -f --tail=100

k8s-port-forward:
	@echo "Port forwarding to 8080..."
	kubectl port-forward svc/$(APP_NAME) -n $(KUBE_NAMESPACE) 8080:80

k8s-scale:
	@echo "Scaling deployment to $(1) replicas..."
	kubectl scale deployment/$(APP_NAME) -n $(KUBE_NAMESPACE) --replicas=$(1)

k8s-kubeconfig:
	@echo "Generating kubeconfig for local development..."
	kubectl config set-context --current --namespace=$(KUBE_NAMESPACE)

k8s-dry-run:
	@echo "Dry run K8s manifests for $(ENV) environment..."
	kustomize build deploy/k8s/overlays/$(ENV) | kubectl apply -f - --dry-run=client -o yaml

k8s-diff:
	@echo "Diff K8s manifests for $(ENV) environment..."
	kustomize build deploy/k8s/overlays/$(ENV) | kubectl diff -f -

.PHONY: all build clean test run docker docker-up help k8s-build k8s-image k8s-push k8s-apply k8s-apply-ns k8s-delete k8s-delete-ns k8s-rollout-status k8s-restart k8s-logs k8s-port-forward k8s-scale k8s-kubeconfig k8s-dry-run k8s-diff

help:
	@echo "Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  build              Build the binary"
	@echo "  build-linux        Build for linux/amd64"
	@echo "  build-darwin       Build for darwin/amd64"
	@echo "  clean              Remove build artifacts"
	@echo ""
	@echo "Test:"
	@echo "  test               Run tests with race detection"
	@echo "  test-short         Run short tests"
	@echo "  test-coverage      Run tests with HTML coverage report"
	@echo ""
	@echo "Run:"
	@echo "  run                Run the service locally"
	@echo "  docker             Build Docker image"
	@echo "  docker-up          Run in Docker with Redis"
	@echo "  docker-compose-up  Start with docker-compose"
	@echo "  docker-compose-down Stop with docker-compose"
	@echo ""
	@echo "Kubernetes (set REGISTRY, IMAGE_TAG, ENV):"
	@echo "  k8s-image          Build Docker image for K8s"
	@echo "  k8s-push           Push image to registry"
	@echo "  k8s-apply          Apply K8s manifests (ENV=staging|production)"
	@echo "  k8s-apply-ns       Create namespace"
	@echo "  k8s-delete         Delete K8s resources"
	@echo "  k8s-delete-ns      Delete namespace"
	@echo "  k8s-rollout-status Check rollout status"
	@echo "  k8s-restart        Restart deployment"
	@echo "  k8s-logs           Follow pod logs"
	@echo "  k8s-port-forward   Port forward to 8080"
	@echo "  k8s-diff           Diff manifests against cluster"
	@echo "  k8s-dry-run        Dry run manifests"
	@echo ""
	@echo "Utilities:"
	@echo "  lint               Run linters"
	@echo "  fmt                Format code"
	@echo "  install            Install binary to GOPATH"
	@echo "  help               Show this help"
