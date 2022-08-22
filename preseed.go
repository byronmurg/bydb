package main

import (
	"context"
	"fmt"
	"flag"
	"log"
	"time"
	"os"
	"io/ioutil"
	s "strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "omanom.com/bydb/proto"
)


var (
	nPartitions = flag.Int("partitions", 10, "number of partitions to preseed")
	nWords = flag.Int("words", 10, "number of words to preseed")
	nWorkers = flag.Int("workers", 10, "number of worker jobs to start")

	gaddresses = []string{
		"localhost:64001",
		"localhost:64002",
		"localhost:64003",
	}

	minDuration int64 = 1e10
	maxDuration int64 = 0
)

func callCrud(client pb.ByDbClient, msg string) *pb.Response {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 100)
	defer cancel()
	r, err := client.Crud(ctx, &pb.Command{Raw: msg})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	return r
}

func loadWords() ([]string, error) {
	var ret []string

	fd, openErr := os.Open("/usr/share/dict/cracklib-small")
	if openErr != nil {
		return ret, openErr
	}

	wordBytes, readErr := ioutil.ReadAll(fd)
	if readErr != nil {
		return ret, readErr
	}

	words := s.Split(string(wordBytes), "\n")

	return words, nil
}

func worker(partitions []string, client pb.ByDbClient, wordChan <-chan string, done chan<- bool) {
	for word := range wordChan {
		for _, part := range partitions {
			//cmd := fmt.Sprintf(`PUT { "id":"%s", "part":"%s", "index":{ "text":"%s" }, "block":{ "hide":"me" }, "categories":[ "%s=%s" ] }`, word, part, word, part, word)
			cmd := fmt.Sprintf(`GET %s %s`, part, word)
			res := callCrud(client, cmd)
			if res.Code != 200 {
				log.Fatalf("recieved code: %d msg: %s", res.Code, res.Document)
			}
			if res.Duration < minDuration {
				minDuration = res.Duration
			}

			if res.Duration > maxDuration {
				maxDuration = res.Duration
			}
		}

		done <- true
	}
}

func createServerClient(connString string) pb.ByDbClient {
	
	// Set up a connection to the server.
	conn, err := grpc.Dial(connString, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	client := pb.NewByDbClient(conn)

	return client
}

func main() {
	flag.Parse()

	allWords, wordErr := loadWords()
	if wordErr != nil { panic(wordErr) }

	words := allWords[0:*nWords]
	partitions := allWords[0:*nPartitions]
	nJobs := *nWords * *nPartitions

	fmt.Printf("partitions: %d words: %d\n", *nPartitions, *nWords)

	var servers []pb.ByDbClient
	for _, addr := range gaddresses {
		server := createServerClient(addr)
		servers = append(servers, server)
	}

	wordChan := make(chan string, *nWords)
	doneChan := make(chan bool, nJobs)

	for i := 0; i < *nWorkers; i++ {
		serverI := i % len(servers)
		go worker(partitions, servers[serverI], wordChan, doneChan)
	}

	start := time.Now()

	for _, word := range words {
		wordChan <- word
	}


	for i := 0; i < *nWords ; i++ {
		<-doneChan
	}

	duration := time.Now().Sub(start)

	fmt.Println("duration", duration)
	fmt.Println("per-op", time.Duration(int(duration) / nJobs))
	fmt.Println("min:", time.Duration(minDuration), "max:", time.Duration(maxDuration))

	/*
	for _, cli := range servers {
		cli.Close()
	}




	callCrud("GET omanom 1234")
	callCrud(`PUT { "part":"omanom", "id":"1234", "index":{ "foo":"bar" }, "block": { "hide":"me" }, "categories": ["active=true"] }`)
	callCrud(`SEARCH omanom "bar"`)
	callCrud(`SEAR omanom "bar"`) //invalid
	*/
}
