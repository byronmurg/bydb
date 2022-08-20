
GOFILES=$(shell find . -name "*.go")

main: main.go | $(GOFILES)
	go build -o $@ $*

clean:
	rm -f main

.PHONY: clean
