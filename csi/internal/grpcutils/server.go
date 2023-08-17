package grpcutils

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/gman0/dummy-fuse-csi/csi/internal/log"

	"google.golang.org/grpc"
)

type (
	grpcEndpoint struct {
		proto string
		addr  string
	}

	Server struct {
		GRPCServer *grpc.Server
		endpoint   grpcEndpoint
	}
)

const (
	unixDomainSocketScheme = "unix://"
	unixDomainSocketProto  = "unix"
)

func NewServer(endpoint string, opt ...grpc.ServerOption) (*Server, error) {
	ep, err := newGRPCEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint %q: %v", endpoint, err)
	}

	return &Server{
		GRPCServer: grpc.NewServer(opt...),
		endpoint:   ep,
	}, nil
}

func newGRPCEndpoint(endpoint string) (grpcEndpoint, error) {
	socketPath := endpoint

	// Strip the protocol off of the endpoint.
	if strings.HasPrefix(endpoint, unixDomainSocketScheme) {
		socketPath = endpoint[len(unixDomainSocketScheme):]
	}

	if !path.IsAbs(socketPath) {
		return grpcEndpoint{},
			errors.New("expected a UNIX domain socket URL unix://<absolute path to socket>")
	}

	return grpcEndpoint{
		proto: unixDomainSocketProto,
		addr:  socketPath,
	}, nil
}

func (s *Server) Serve() error {
	if s.endpoint.proto == unixDomainSocketProto {
		// Try to delete any existing socket at the endpoint path before continuing.
		if err := tryRemoveSocket(s.endpoint.addr); err != nil {
			return fmt.Errorf("failed to existing UNIX domain socket %q: %v",
				s.endpoint.addr, err)
		}
	}

	listener, err := net.Listen(s.endpoint.proto, s.endpoint.addr)
	if err != nil {
		return fmt.Errorf("listen failed: %v", err)
	}

	log.Infof("Listening for connections on %s", listener.Addr())

	return s.GRPCServer.Serve(listener)
}

func tryRemoveSocket(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("not a UNIX domain socket")
	}

	err = os.Remove(p)
	if err != nil && os.IsNotExist(err) {
		return nil
	}

	return err
}
