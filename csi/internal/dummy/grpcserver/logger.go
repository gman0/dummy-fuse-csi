package grpcserver

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
)

var (
	callCounter uint64
)

func logRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	callID := atomic.AddUint64(&callCounter, 1)

	log.Printf("[ID:%d] GRPC call: %s", callID, info.FullMethod)
	log.Printf("[ID:%d] GRPC request: %s", callID, protosanitizer.StripSecrets(req))

	resp, err := handler(ctx, req)

	if err != nil {
		log.Printf("[ID:%d] GRPC error: %v", callID, err)
	} else {
		log.Printf("[ID:%d] GRPC response: %s", callID, protosanitizer.StripSecrets(resp))
	}

	return resp, err
}
