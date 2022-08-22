package main

import (
	"context"
	"fmt"
	"flag"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "omanom.com/bydb/proto"
)

var (
	addr = flag.String("addr", "localhost:64001", "the address to connect to")
	client pb.ByDbClient = nil
)

func callCrud(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := client.Crud(ctx, &pb.Command{Raw: msg})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
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



	callCrud("GET omanom 1234")
	callCrud(`PUT { "part":"omanom", "id":"1234", "index":{ "foo":"bar" }, "block": { "hide":"me" }, "categories": ["active=true"] }`)
	callCrud(`SEARCH omanom "bar"`)
	callCrud(`SEAR omanom "bar"`) //invalid
}
