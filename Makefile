.PHONY: proto
proto:
	buf generate --template ./api/buf.gen.yaml --output ./api ./api

.PHONY: functests
functests:
	go run github.com/onsi/ginkgo/v2/ginkgo run -v ./test/functional