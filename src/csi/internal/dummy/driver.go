package dummy

import (
	"log"

	"dummy-fuse-csi/internal/dummy/grpcserver"
	"dummy-fuse-csi/internal/dummy/version"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type Driver struct {
	*DriverOpts

	ids csi.IdentityServer
	ns  csi.NodeServer
}

func NewDriver(opts *DriverOpts) (*Driver, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	log.Println("Driver:", opts.Name)
	log.Println("Driver version:", version.Version)

	d := &Driver{
		DriverOpts: opts,
	}

	if opts.MountCachePath != "" {
		log.Println("Attempting to re-mount volumes")
		remountStaged(opts.MountCachePath)
		remountPublished(opts.MountCachePath)
	}

	// Initialize Identity Service
	d.ids = newIdentityService(d)

	// Initialize Node Service
	d.ns = newNodeService(d)

	return d, nil
}

func (d *Driver) Run() error {
	s := grpcserver.New(d.Endpoint)

	log.Println("Registering Identity server")
	csi.RegisterIdentityServer(s.Server, d.ids)

	log.Println("Registering Node server")
	csi.RegisterNodeServer(s.Server, d.ns)

	return s.Serve()
}
