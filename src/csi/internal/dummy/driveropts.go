package dummy

import (
	"errors"
)

type DriverOpts struct {
	Endpoint string
	Name     string
	NodeID   string

	MountCachePath string
}

func (o *DriverOpts) validate() error {
	if o.Endpoint == "" {
		return errors.New("driver endpoint not set")
	}

	if o.Name == "" {
		return errors.New("driver name not set")
	}

	if o.NodeID == "" {
		return errors.New("node ID not set")
	}

	return nil
}
