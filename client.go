package main

import (
	"context"
	"fmt"
	"flag"
	"log"
	"time"
	"regexp"
	"errors"
	"encoding/json"
	s "strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "omanom.com/bydb/proto"

	"github.com/manifoldco/promptui"
)

var (
	addr = flag.String("addr", "localhost:64001", "the address to connect to")
	client pb.ByDbClient = nil
)

func callCrud(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()
	r, err := client.Crud(ctx, &pb.Command{Raw: msg})
	if err != nil {
		log.Fatalf("crud error: %v", err)
	}
	fmt.Print(r.GetDocument(), "\n")
}

type CommandEntry struct {
	Prefix string
	Pattern string
}

func main() {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client = pb.NewByDbClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := client.Hello(ctx, &pb.Greeting{Msg: "I'm the client"})
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	fmt.Printf("Server: %s\n", r.GetMsg())

	commands := []CommandEntry{}
	commandJsErr := json.Unmarshal([]byte(r.GetCommandlist()), &commands)
	if commandJsErr != nil { panic(commandJsErr) }

	validate := func(input string) error {
		if s.ToLower(input) == "exit" {
			return nil
		}

		for _, cmd := range commands {
			if s.HasPrefix(input, cmd.Prefix) {
				r := regexp.MustCompile(cmd.Pattern)
				if ! r.MatchString(input) {
					return errors.New("invalid format for command "+ cmd.Prefix)
				} else {
					return nil
				}
			}
		}

		return errors.New("no matching command found")
	}

	prompt := promptui.Prompt{
		Label: "$",
		Validate: validate,
	}

	for {
		result, err := prompt.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		if "exit" == s.ToLower(result) {
			break
		}

		callCrud(result)
	}
}
