BINARY := sfsymbols
DIST := dist
BINDIR ?= /usr/local/bin

.PHONY: build universal install clean clean-cache test

build:
	go build -o $(BINARY) .

# macOS universal binary (arm64 + x86_64)
universal:
	mkdir -p $(DIST)
	GOOS=darwin GOARCH=arm64 go build -o $(DIST)/$(BINARY)-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o $(DIST)/$(BINARY)-amd64 .
	lipo -create -output $(DIST)/$(BINARY) $(DIST)/$(BINARY)-arm64 $(DIST)/$(BINARY)-amd64
	lipo -info $(DIST)/$(BINARY)
	rm $(DIST)/$(BINARY)-arm64 $(DIST)/$(BINARY)-amd64

# Override dir with `make install BINDIR=~/.local/bin` to avoid sudo.
install: universal
	install -m 0755 $(DIST)/$(BINARY) $(BINDIR)/$(BINARY)

test: build
	./$(BINARY) check bus.fill car.ferry.fill && ./$(BINARY) search car --limit 3

clean:
	rm -rf $(BINARY) $(DIST)

clean-cache:
	rm -rf "$(HOME)/Library/Caches/sfsymbols"
