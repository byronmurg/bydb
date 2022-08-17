
FILES=(main.go command.go document.go statemachine.go store.go zipper.go)

main: *.go
	go build -o $@ $*
