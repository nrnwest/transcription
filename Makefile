.PHONY: build run test clean

TRANSCRTIPTION_BINARY=transcription-run

# Build the application binary
build:
	go build -o $(TRANSCRTIPTION_BINARY) ./cmd/transcription/

# Run with an arbitrary language and directory
# Example: make run LANG=de DIR=/Users/nrnwest/project/_my/transcription/file_temp
# LANG defaults to auto; DIR is required.
LANG ?= auto
run:
	@if [ -z "$(DIR)" ]; then \
		echo "Error: DIR=/path/to/videos is required"; \
		echo "Example: make run LANG=de DIR=/path/to/videos"; \
		exit 1; \
	fi
	./$(TRANSCRTIPTION_BINARY) -lang $(LANG) $(DIR)

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f $(TRANSCRTIPTION_BINARY)
	rm -rf tmp/
