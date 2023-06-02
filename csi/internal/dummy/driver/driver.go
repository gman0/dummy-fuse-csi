package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gman0/dummy-fuse-csi/csi/internal/dummy/identity"
	"github.com/gman0/dummy-fuse-csi/csi/internal/dummy/node"
	"github.com/gman0/dummy-fuse-csi/csi/internal/grpcutils"
	"github.com/gman0/dummy-fuse-csi/csi/internal/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/validation"
)

type (
	// Service role name.
	ServiceRole string

	// Opts holds init-time driver configuration.
	Opts struct {
		// DriverName is the name of this CSI driver that's then
		// advertised via NodeGetPluginInfo RPC.
		DriverName string

		// CSIEndpoint is URL path to the UNIX socket where the driver
		// will serve requests.
		CSIEndpoint string

		// NodeID is unique identifier of the node on which this
		// CVMFS CSI node plugin pod is running.
		NodeID string

		// Role under which will the driver operate.
		Roles map[ServiceRole]bool
	}

	// Driver holds CVMFS-CSI driver runtime state.
	Driver struct {
		*Opts
	}
)

const (
	IdentityServiceRole   = "identity"   // Enable identity service role.
	NodeServiceRole       = "node"       // Enable node service role.
	ControllerServiceRole = "controller" // Enable controller service role.
)

const (
	// dummy-fuse-csi driver name.
	DefaultName = "dummy-fuse-csi.csi.cern.ch"

	// Maximum driver name length as per CSI spec.
	maxDriverNameLength = 63
)

var (
	errTimeout = errors.New("timed out waiting for condition")
)

func (o *Opts) validate() error {
	required := func(name, value string) error {
		if value == "" {
			return fmt.Errorf("%s is a required parameter", name)
		}

		return nil
	}

	if err := required("drivername", o.DriverName); err != nil {
		return err
	}

	if len(o.DriverName) > maxDriverNameLength {
		return fmt.Errorf("driver name too long: is %d characters, maximum is %d",
			len(o.DriverName), maxDriverNameLength)
	}

	// As per CSI spec, driver name must follow DNS format.
	if errMsgs := validation.IsDNS1123Subdomain(strings.ToLower(o.DriverName)); len(errMsgs) > 0 {
		return fmt.Errorf("driver name is invalid: %v", errMsgs)
	}

	if err := required("endpoint", o.CSIEndpoint); err != nil {
		return err
	}

	if err := required("nodeid", o.NodeID); err != nil {
		return err
	}

	return nil
}

// New creates a new instance of Driver.
func New(opts *Opts) (*Driver, error) {
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("invalid driver options: %v", err)
	}

	return &Driver{
		Opts: opts,
	}, nil
}

func setupIdentityServiceRole(s *grpc.Server, d *Driver) error {
	log.Debugf("Registering Identity server")
	csi.RegisterIdentityServer(
		s,
		identity.New(
			d.DriverName,
			d.Opts.Roles[ControllerServiceRole],
		),
	)

	return nil
}

func setupNodeServiceRole(s *grpc.Server, d *Driver) error {
	ns := node.New(d.NodeID)

	caps, err := ns.NodeGetCapabilities(
		context.TODO(),
		&csi.NodeGetCapabilitiesRequest{},
	)
	if err != nil {
		return fmt.Errorf("failed to get Node server capabilities: %v", err)
	}

	log.Debugf("Registering Node server with capabilities %+v", caps.GetCapabilities())
	csi.RegisterNodeServer(s, ns)

	return nil
}

// Run starts CSI services and blocks.
func (d *Driver) Run() error {
	log.Infof("Driver: %s", d.DriverName)

	s, err := grpcutils.NewServer(d.CSIEndpoint, grpc.UnaryInterceptor(grpcLogger))
	if err != nil {
		return fmt.Errorf("failed to create GRPC server: %v", err)
	}

	if d.Opts.Roles[IdentityServiceRole] {
		if err = setupIdentityServiceRole(s.GRPCServer, d); err != nil {
			return fmt.Errorf("failed to setup identity service role: %v", err)
		}
	}

	if d.Opts.Roles[NodeServiceRole] {
		if err = setupNodeServiceRole(s.GRPCServer, d); err != nil {
			return fmt.Errorf("failed to setup node service role: %v", err)
		}
	}

	return s.Serve()
}
