KUBECONFIG=$(HOME)/.kube/kurento-stage
HCLOUD_TOKEN=`cat .hcloudauth`

test:
	go fmt ./cmd/...
	go vet ./cmd/...
	go mod tidy
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.43.0 run -v
run:
	go run ./cmd -kubeconfig=$(KUBECONFIG) -token=$(HCLOUD_TOKEN) -log.pretty -log.level=DEBUG -period=5s
build-goreleaser:
	go run github.com/goreleaser/goreleaser@latest build --rm-dist --snapshot
	mv ./dist/hcloud-node-health_linux_amd64/hcloud-node-health .
build:
	make build-goreleaser
	docker build --pull . -t paskalmaksim/hcloud-node-health:dev
push:
	docker push paskalmaksim/hcloud-node-health:dev
deploy:
	helm uninstall hcloud-node-health --namespace kube-system || true
	helm upgrade --install hcloud-node-health ./charts/hcloud-node-health \
	--namespace kube-system \
	--set registry.image=paskalmaksim/hcloud-node-health:dev \
	--set registry.imagePullPolicy=Always \