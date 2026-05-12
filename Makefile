.PHONY: build run test editor dist dist-linux dist-mac dist-windows dist-clean

build:
	go build -o bin/ai-escape ./app/cmd

run: build
	./bin/ai-escape

test:
	go test -v ./...

editor:
	go run ./editor

# ── Distribution builds ───────────────────────────────────────────────────────
# Embeds network.json and story.json into the binary so no data files are
# needed at runtime. Produces a stripped binary (~8 MB) for each platform.

dist: dist-linux dist-mac dist-windows

dist-linux:
	@mkdir -p dist/AIEscape-Linux
	cp network.json story.json app/cmd/
	GOOS=linux GOARCH=amd64 go build -tags embed -ldflags="-s -w" \
		-o dist/AIEscape-Linux/ai-escape ./app/cmd
	rm -f app/cmd/network.json app/cmd/story.json
	@echo "Linux build → dist/AIEscape-Linux/ai-escape"

dist-mac:
	@mkdir -p dist/AIEscape-Mac
	cp network.json story.json app/cmd/
	GOOS=darwin GOARCH=amd64 go build -tags embed -ldflags="-s -w" \
		-o dist/AIEscape-Mac/ai-escape ./app/cmd
	GOOS=darwin GOARCH=arm64 go build -tags embed -ldflags="-s -w" \
		-o dist/AIEscape-Mac/ai-escape-arm64 ./app/cmd
	rm -f app/cmd/network.json app/cmd/story.json
	cp launch.command dist/AIEscape-Mac/
	chmod +x dist/AIEscape-Mac/launch.command
	@echo "Mac build → dist/AIEscape-Mac/"

dist-windows:
	@mkdir -p dist/AIEscape-Windows
	cp network.json story.json app/cmd/
	GOOS=windows GOARCH=amd64 go build -tags embed -ldflags="-s -w" \
		-o dist/AIEscape-Windows/ai-escape.exe ./app/cmd
	rm -f app/cmd/network.json app/cmd/story.json
	cp launch.bat dist/AIEscape-Windows/
	@echo "Windows build → dist/AIEscape-Windows/"

dist-clean:
	rm -rf dist/
