.PHONY: proto
proto:
	@buf generate --template ./api/buf.gen.yaml --output ./api ./api

.PHONY: functests
functests:
	@go run github.com/onsi/ginkgo/v2/ginkgo run -v -p $(ARGS) ./test/functional

.PHONY: lint
lint:
	@golangci-lint run -v

.PHONY: fmt
fmt:
	@find . -type f -name '*.go' \
       -not -path './vendor/*' \
       -not -name '*.pb.go' \
       -exec gofmt -w {} +