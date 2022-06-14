package main

import (
	"fmt"
	"github.com/go-co-op/gocron"
	"google.golang.org/grpc"
	grpc_proto "grpc-research-server/grpc"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type ChatServer struct {
	grpc_proto.UnimplementedChatServer
	mu         sync.Mutex
	chats 		map[string][]*grpc_proto.ChatContent
}

func (s *ChatServer) StreamChat(stream grpc_proto.Chat_StreamChatServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		key := serialize(in)

		s.mu.Lock()
		s.chats[key] = append(s.chats[key], in)
		// Note: this copy prevents blocking other clients while serving this one.
		// We don't need to do a deep copy, because elements in the slice are
		// insert-only and never modified.
		rn := make([]*grpc_proto.ChatContent, len(s.chats[key]))
		copy(rn, s.chats[key])
		s.mu.Unlock()

		for _, note := range rn {
			if err := stream.Send(note); err != nil {
				return err
			}
		}
	}
}

func serialize(content *grpc_proto.ChatContent) string {
	return fmt.Sprintf("%d", content.Content)
}

func newServer() *ChatServer {
	s := &ChatServer{chats: make(map[string][]*grpc_proto.ChatContent)}

	return s
}

func main()  {
	envPort := os.Getenv("PORT")
	if envPort == "" {
		envPort = "50051"
	}

	lis, err := net.Listen("tcp", ":" + envPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	grpc_proto.RegisterChatServer(grpcServer, newServer())
	go func() {
		s := gocron.NewScheduler(time.UTC)
		s.Every(15).Seconds().Do(func() {
			log.Printf("server listening at %v", lis.Addr())
		})

		s.StartAsync()

	}()
	grpcServer.Serve(lis)
}