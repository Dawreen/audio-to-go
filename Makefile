.PHONY: run build clean

run:
	-pkill -f "audio-to-go" 2>/dev/null; sleep 1
	CGO_ENABLED=0 go run ./cmd/audio-to-go

build:
	CGO_ENABLED=0 go build -o audio-to-go ./cmd/audio-to-go

clean:
	rm -f audio-to-go
	go clean -cache
