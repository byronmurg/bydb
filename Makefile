
GOFILES=$(shell find . -name "*.go")

all: server client preseed
	@echo building

server: server.go $(GOFILES)
	go build -o $@ $<

client: client.go $(GOFILES)
	go build -o $@ $<

preseed: preseed.go $(GOFILES)
	go build -o $@ $<

clean:
	rm -f server client preseed

.PHONY: clean all
