
GOFILES=$(shell find . -name "*.go")

all: server client
	@echo building

server: server.go $(GOFILES)
	go build -o $@ $<

client: client.go $(GOFILES)
	go build -o $@ $<

clean:
	rm -f server client

.PHONY: clean all
