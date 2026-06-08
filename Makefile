.PHONY: proto
proto:
	buf generate --template ./api/buf.gen.yaml --output ./api ./api
