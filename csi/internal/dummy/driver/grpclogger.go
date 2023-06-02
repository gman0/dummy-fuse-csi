package driver

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/gman0/dummy-fuse-csi/csi/internal/log"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
)

var (
	// Counter value used for pairing up GRPC call and response log messages.
	grpcCallCounter uint64
)

func fmtGRPCLogMsg(grpcCallID uint64, msg string) string {
	return fmt.Sprintf("Call-ID %d: %s", grpcCallID, msg)
}

func grpcLogger(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	grpcCallID := atomic.AddUint64(&grpcCallCounter, 1)

	log.DebugfWithContext(ctx, fmtGRPCLogMsg(grpcCallID, fmt.Sprintf("Call: %s", info.FullMethod)))
	log.DebugfWithContext(ctx, fmtGRPCLogMsg(grpcCallID, fmt.Sprintf("Request: %s", protosanitizer.StripSecrets(req))))

	resp, err := handler(ctx, req)
	if err != nil {
		log.ErrorfWithContext(ctx, fmtGRPCLogMsg(grpcCallID, fmt.Sprintf("Error: %v", err)))
	} else {
		log.DebugfWithContext(ctx, fmtGRPCLogMsg(grpcCallID, fmt.Sprintf("Response: %s", protosanitizer.StripSecrets(resp))))
	}

	return resp, err
}
