.PHONY: build test race vet check fuzz

build:
	go build -trimpath -o bin/miwu ./cmd/miwu

test:
	go test -timeout=90s ./...

race:
	go test -race -timeout=90s ./...

vet:
	go vet ./...

check: test race vet build

fuzz:
	go test ./internal/miot -run='^$$' -fuzz=FuzzResponseEnvelope -fuzztime=3s -parallel=2
	go test ./internal/miot/auth -run='^$$' -fuzz=FuzzAuthenticationResponse -fuzztime=3s -parallel=2
	go test ./internal/miot/spec -run='^$$' -fuzz=FuzzTranslationKeys -fuzztime=3s -parallel=2
	go test ./internal/miot/urn -run='^$$' -fuzz=FuzzURNRoundTrip -fuzztime=3s -parallel=2
