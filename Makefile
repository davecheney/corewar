.PHONY: run test snapshot badge generate

run:
	go run ./cmd/corewar

test:
	go test ./...
	go vet ./...

generate:
	go generate ./...

snapshot:
	go run ./cmd/snapshot

# Produces a UF2 for the Gopher Badge in the current directory.
badge:
	tinygo build -target gopher-badge -o corewar.uf2 ./cmd/corewar-badge
