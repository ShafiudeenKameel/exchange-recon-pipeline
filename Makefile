.PHONY: test bronze gold

test:
	GOPROXY=https://proxy.golang.org,direct GOSUMDB=off go test ./...

bronze:
	GOPROXY=https://proxy.golang.org,direct GOSUMDB=off go run ./cmd/bronze

gold:
	GOPROXY=https://proxy.golang.org,direct GOSUMDB=off go run ./cmd/gold
