package main

import (
	"bufio"
	"context"
	"fmt"
	"flag"
	"os"
	"log"
	"time"
	s "strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "omanom.com/bydb/proto"
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



	cli := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("$ ")

		rawStr, err := cli.ReadString('\n')
		str := s.TrimSpace(rawStr)

		if err != nil {
			fmt.Print(err)
			break
		}

		if s.ToLower(str) == "exit" {
			break
		}

		switch str {
		case "":
			continue
		default:
			callCrud(str)
		}
	}
}
