package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gman0/dummy-fuse-csi/csi/internal/dummy/driver"
	"github.com/gman0/dummy-fuse-csi/csi/internal/log"
	V "github.com/gman0/dummy-fuse-csi/csi/internal/version"

	"k8s.io/klog/v2"
)

type rolesFlag []driver.ServiceRole

func (rf rolesFlag) String() string {
	return fmt.Sprintf("%v", []driver.ServiceRole(rf))
}

var (
	knownServiceRoles = map[driver.ServiceRole]struct{}{
		driver.IdentityServiceRole:   {},
		driver.NodeServiceRole:       {},
		driver.ControllerServiceRole: {},
	}
)

func (rf *rolesFlag) Set(newRoleFlag string) error {
	for _, part := range strings.Split(newRoleFlag, ",") {
		if _, ok := knownServiceRoles[driver.ServiceRole(part)]; !ok {
			return fmt.Errorf("unknown role %s", part)
		}

		*rf = append(*rf, driver.ServiceRole(part))
	}

	return nil
}

var (
	endpoint   = flag.String("endpoint", fmt.Sprintf("unix:///var/lib/kubelet/plugins/%s/csi.sock", driver.DefaultName), "CSI endpoint.")
	driverName = flag.String("drivername", driver.DefaultName, "Name of the driver.")
	nodeId     = flag.String("nodeid", "", "Node id.")
	version    = flag.Bool("version", false, "Print driver version and exit.")
	roles      rolesFlag
)

func main() {
	// Handle flags and initialize logging.

	flag.Var(&roles, "role", "Enable driver service role (comma-separated list or repeated --role flags). Allowed values are: 'identity', 'node', 'controller'.")

	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Exitf("failed to set logtostderr flag: %v", err)
	}
	flag.Parse()

	if *version {
		fmt.Println("CVMFS CSI plugin version", V.FullVersion())
		os.Exit(0)
	}

	// Initialize and run the driver.

	log.Infof("Dummy-FUSE CSI plugin version %s", V.FullVersion())
	log.Infof("Command line arguments %v", os.Args)

	driverRoles := make(map[driver.ServiceRole]bool, len(roles))
	for _, role := range roles {
		driverRoles[role] = true
	}

	driver, err := driver.New(&driver.Opts{
		DriverName:  *driverName,
		CSIEndpoint: *endpoint,
		NodeID:      *nodeId,
		Roles:       driverRoles,
	})

	if err != nil {
		log.Fatalf("Failed to initialize the driver: %v", err)
	}

	err = driver.Run()
	if err != nil {
		log.Fatalf("Failed to run the driver: %v", err)
	}

	os.Exit(0)
}
