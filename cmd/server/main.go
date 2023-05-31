package main

import (
	"log"
	"net"

	"github.com/silvan-talos/mock/http"
	"github.com/silvan-talos/mock/mocking"
)

func main() {
	mocker := mocking.NewMocker()
	ms := mocking.NewService(mocker)
	server := http.NewServer(ms)
	lis, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("failed to create listener: %w, address:%s\n", err, "localhost:8080")
	}
	err = server.Serve(lis)
	if err != nil {
		log.Fatal("server error:", err)
	}
}
