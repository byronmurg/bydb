package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	s "strings"
	"time"

	"math/rand"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "omanom.com/bydb/proto"
)

var (
	nPartitions = flag.Int("partitions", 10, "number of partitions to preseed")
	nWords      = flag.Int("words", 10, "number of words to preseed")
	nWorkers    = flag.Int("workers", 10, "number of worker jobs to start")

	gaddresses = []string{
		"localhost:64001",
		"localhost:64002",
		"localhost:64003",
	}
)

func callCrud(client pb.ByDbClient, msg string) *pb.Response {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
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

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })

	return words, nil
}

type TestResults struct {
	MaxDuration int64
	MinDuration int64
	Duration    time.Duration
}

type testFormatCbk = func(string, string) string

type TestBatch struct {
	nWorkers      int
	partitions    []string
	words         []string
	formatCommand testFormatCbk
	servers       []pb.ByDbClient
}

func (s *TestBatch) Run() *TestResults {
	//@TODO this

	start := time.Now()

	wordsPerWorker := len(s.words) / s.nWorkers
	var wordBuf = s.words

	doneChan := make(chan bool, s.nWorkers)

	res := TestResults{
		MinDuration: 10e6,
		MaxDuration: 0,
	}

	for i := 0; i < s.nWorkers; i++ {
		serverNo := i % len(s.servers)
		server := s.servers[serverNo]
		words := wordBuf[0:wordsPerWorker]
		wordBuf = wordBuf[wordsPerWorker:len(wordBuf)]

		var MinDuration int64 = 10e6
		var MaxDuration int64 = 0

		go func() {
			for _, part := range s.partitions {
				for _, word := range words {

					cmdStr := s.formatCommand(part, word)
					res := callCrud(server, cmdStr)
					if res.Code != 200 {
						log.Fatalf("recieved code: %d msg: %s cmd: %s", res.Code, res.Document, cmdStr)
					}

					if res.Duration < MinDuration {
						MinDuration = res.Duration
					}

					if res.Duration > MaxDuration {
						MaxDuration = res.Duration
					}
				}
			}

			if MinDuration < res.MinDuration {
				res.MinDuration = MinDuration
			}

			if MaxDuration > res.MaxDuration {
				res.MaxDuration = MaxDuration
			}

			doneChan <- true

		}()
	}

	// Wait for all workers to complete
	for i := 0; i < s.nWorkers; i++ {
		<-doneChan
	}

	res.Duration = time.Now().Sub(start)

	return &res
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

func PrintResults(res *TestResults) {
	fmt.Println("duration", res.Duration)
	//fmt.Println("per-op", time.Duration(int(duration) / nJobs))
	fmt.Println("min:", time.Duration(res.MinDuration), "max:", time.Duration(res.MaxDuration))
}

func main() {
	flag.Parse()

	allWords, wordErr := loadWords()
	if wordErr != nil {
		panic(wordErr)
	}

	words := allWords[0:*nWords]
	partitions := allWords[0:*nPartitions]
	total := *nPartitions * *nWords

	fmt.Printf("partitions: %d words: %d total: %d\n", *nPartitions, *nWords, total)

	var servers []pb.ByDbClient
	for _, addr := range gaddresses {
		server := createServerClient(addr)
		servers = append(servers, server)
	}

	createTime := time.Now().UnixMilli()
	updateTime := time.Now().UnixMilli()

	postBatch := TestBatch{
		formatCommand: func(part string, word string) string {
			return fmt.Sprintf(`POST { "id":"%s", "part":"%s", "index":{ "text":"%s" }, "block":{ "hide":"me" }, "categories":[ "%s=%s" ], "updated":%d, "created":%d }`, word, part, word, part, word, createTime, createTime)
		},
		words:      words,
		partitions: partitions,
		nWorkers:   *nWorkers,
		servers:    servers,
	}

	fmt.Println("POST")
	PrintResults(postBatch.Run())

	getBatch := TestBatch{
		formatCommand: func(part string, word string) string {
			return fmt.Sprintf(`GET %s %s`, part, word)
		},
		words:      words,
		partitions: partitions,
		nWorkers:   *nWorkers,
		servers:    servers,
	}

	fmt.Println("GET")
	PrintResults(getBatch.Run())

	putBatch := TestBatch{
		formatCommand: func(part string, word string) string {
			return fmt.Sprintf(`PUT %d { "id":"%s", "part":"%s", "index":{ "text":"%s" }, "block":{ "hide":"me" }, "categories":[ "%s=%s" ], "updated":%d, "created":%d }`, createTime, word, part, word, part, word, updateTime, updateTime)
		},
		words:      words,
		partitions: partitions,
		nWorkers:   *nWorkers,
		servers:    servers,
	}

	fmt.Println("PUT")
	PrintResults(putBatch.Run())

	delBatch := TestBatch{
		formatCommand: func(part string, word string) string {
			return fmt.Sprintf(`DEL %s %s %d`, part, word, updateTime)
		},
		words:      words,
		partitions: partitions,
		nWorkers:   *nWorkers,
		servers:    servers,
	}

	fmt.Println("DEL")
	PrintResults(delBatch.Run())
}
