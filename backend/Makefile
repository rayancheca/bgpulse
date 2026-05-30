BINARY := bgpulse
PKG    := ./...
COVER  := coverage.out

# Packages that carry the core routing-security logic; must hold >=80% coverage.
CORE := ./internal/valleyfree/... ./internal/rpki/... ./internal/relationships/... \
        ./internal/classify/... ./internal/synth/... ./internal/topology/...

.PHONY: build run-demo test test-race cover tidy vet clean

build:
	go build -o bin/$(BINARY) ./cmd/bgpulse

run-demo: build
	./bin/$(BINARY) -mode demo -listen :8080 -log-format text

test:
	go test $(PKG)

test-race:
	go test -race $(PKG)

cover:
	go test -coverprofile=$(COVER) $(CORE)
	go tool cover -func=$(COVER) | tail -1

vet:
	go vet $(PKG)

tidy:
	go mod tidy

clean:
	rm -rf bin $(COVER)
