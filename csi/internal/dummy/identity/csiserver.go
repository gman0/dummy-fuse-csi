package identity

import (
	"context"

	"github.com/gman0/dummy-fuse-csi/csi/internal/version"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

// Server implements csi.IdentityServer interface.
type Server struct {
	driverName string
	caps       []*csi.PluginCapability
}

var _ csi.IdentityServer = (*Server)(nil)

func New(driverName string, hasControllerService bool) *Server {
	supportedRpcs := []csi.PluginCapability_Service_Type{
		csi.PluginCapability_Service_UNKNOWN,
	}

	if hasControllerService {
		supportedRpcs = append(supportedRpcs, csi.PluginCapability_Service_CONTROLLER_SERVICE)
	}

	var caps []*csi.PluginCapability
	for _, c := range supportedRpcs {
		caps = append(caps, &csi.PluginCapability{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: c,
				},
			},
		})
	}

	return &Server{
		driverName: driverName,
		caps:       caps,
	}
}

func (srv *Server) GetPluginInfo(
	ctx context.Context,
	req *csi.GetPluginInfoRequest,
) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          srv.driverName,
		VendorVersion: version.Version(),
	}, nil
}

func (srv *Server) GetPluginCapabilities(
	ctx context.Context,
	req *csi.GetPluginCapabilitiesRequest,
) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: srv.caps,
	}, nil
}

func (srv *Server) Probe(
	ctx context.Context,
	req *csi.ProbeRequest,
) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}
