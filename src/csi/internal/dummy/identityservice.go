package dummy

import (
	"context"

	"dummy-fuse-csi/internal/dummy/version"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type identityService struct {
	d    *Driver
	caps []*csi.PluginCapability
}

func newIdentityService(d *Driver) csi.IdentityServer {
	supportedRpcs := []csi.PluginCapability_Service_Type{
		csi.PluginCapability_Service_UNKNOWN,
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

	return &identityService{
		d:    d,
		caps: caps,
	}
}

func (ids *identityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          ids.d.Name,
		VendorVersion: version.Version,
	}, nil
}

func (ids *identityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: ids.caps,
	}, nil
}

func (ids *identityService) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}
