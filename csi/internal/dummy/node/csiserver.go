package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	goexec "os/exec"
	"sync"

	"github.com/gman0/dummy-fuse-csi/csi/internal/exec"
	"github.com/gman0/dummy-fuse-csi/csi/internal/log"
	"github.com/gman0/dummy-fuse-csi/csi/internal/mountutils"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var automountDaemonRunning sync.Once

func reconcileStagingPath(stagingPath string) error {

	// / # cat /etc/dummy.autofs
	// /opt -fstype=fuse3 :dummy-fuse
	// # ln -s /bin/dummy-fuse /bin/mount.dummy-fuse

	return reconcileMount(stagingPath, func(mountpoint string) error {
		// Write autofs map file.

		mapFileContentsFmt := `
%s -fstype=fuse3 :dummy-fuse
`

		if err := os.WriteFile("/etc/dummy.autofs", []byte(fmt.Sprintf(mapFileContentsFmt, stagingPath)), 0655); err != nil {
			return err
		}

		// Start automount daemon.

		if err := exec.RunAndLogCombined(goexec.Command("automount", "-f", "--debug")); err != nil {
			return fmt.Errorf("failed to run automonut: %v", err)
		}

		automountDaemonRunning.Do(func() {
			go func() {
				if err := exec.RunAndLogCombined(goexec.Command("automount", "-f", "--debug")); err != nil {
					log.Fatalf("Failed to run automount daemon: %v", err)
				}
			}()
		})

		// Share the mount.

		if err := shareMount(mountpoint); err != nil {
			return err
		}

		return nil
	})
}

func reconcilePublishPath(stagingPath, publishPath string) error {
	return reconcileMount(publishPath, func(mountpoint string) error {
		return slaveRecursiveBind(stagingPath, mountpoint)
	})
}

// Server implements csi.NodeServer interface.
type Server struct {
	nodeID string
	caps   []*csi.NodeServiceCapability
}

var (
	_ csi.NodeServer = (*Server)(nil)
)

func New(nodeID string) *Server {
	enabledCaps := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	}

	var caps []*csi.NodeServiceCapability
	for _, c := range enabledCaps {
		caps = append(caps, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: c,
				},
			},
		})
	}

	return &Server{
		nodeID: nodeID,
		caps:   caps,
	}
}

func (srv *Server) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest,
) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: srv.caps,
	}, nil
}

func (srv *Server) NodeGetInfo(
	ctx context.Context,
	req *csi.NodeGetInfoRequest,
) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: srv.nodeID,
	}, nil
}

func (srv *Server) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest,
) (*csi.NodePublishVolumeResponse, error) {
	if err := validateNodePublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	stagingPath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()

	if err := os.MkdirAll(targetPath, 0700); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to create mountpoint directory at %s: %v", targetPath, err)
	}

	// Reconcile staging and publish volume paths.

	if err := reconcileStagingPath(stagingPath); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to reconcile mountpoint %s: %v", stagingPath, err)
	}

	if err := reconcilePublishPath(stagingPath, targetPath); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to reconcile mountpoint %s: %v", targetPath, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (srv *Server) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := validateNodeUnpublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	targetPath := req.GetTargetPath()

	// Unmount targetPath and remove the mountpoint (required by the CSI spec).

	if err := mountutils.Unmount(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to unmount %s: %v", targetPath, err)
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (srv *Server) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest,
) (*csi.NodeStageVolumeResponse, error) {
	if err := validateNodeStageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	stagingPath := req.GetStagingTargetPath()

	if err := reconcileStagingPath(stagingPath); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to reconcile mountpoint %s: %v", stagingPath, err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (srv *Server) NodeUnstageVolume(
	ctx context.Context,
	req *csi.NodeUnstageVolumeRequest,
) (*csi.NodeUnstageVolumeResponse, error) {
	if err := validateNodeUnstageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	stagingPath := req.GetStagingTargetPath()

	if err := mountutils.Unmount(req.GetStagingTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to unmount %s: %v", stagingPath, err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (srv *Server) NodeGetVolumeStats(
	ctx context.Context,
	req *csi.NodeGetVolumeStatsRequest,
) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (srv *Server) NodeExpandVolume(
	ctx context.Context,
	req *csi.NodeExpandVolumeRequest,
) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func validateNodePublishVolumeRequest(req *csi.NodePublishVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}

	if req.GetVolumeCapability().GetBlock() != nil {
		return errors.New("volume access type Block is unsupported")
	}

	if req.GetVolumeCapability().GetMount() == nil {
		return errors.New("volume access type must by Mount")
	}

	if req.GetTargetPath() == "" {
		return errors.New("volume target path missing in request")
	}

	// We're not checking for staging target path, as older versions
	// of the driver didn't support STAGE_UNSTAGE_VOLUME capability.

	if req.GetVolumeCapability().GetAccessMode().GetMode() !=
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
		return fmt.Errorf("volume access mode must be ReadOnlyMany")
	}

	if volCtx := req.GetVolumeContext(); len(volCtx) > 0 {
		unsupportedVolumeParams := []string{"hash", "tag"}

		for _, volParam := range unsupportedVolumeParams {
			if _, ok := volCtx[volParam]; ok {
				return fmt.Errorf("volume parameter %s is not supported, please use clientConfig instead", volParam)
			}
		}
	}

	return nil
}

func validateNodeUnpublishVolumeRequest(req *csi.NodeUnpublishVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetTargetPath() == "" {
		return errors.New("target path missing in request")
	}

	return nil
}

func validateNodeStageVolumeRequest(req *csi.NodeStageVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}

	if req.GetVolumeCapability().GetBlock() != nil {
		return errors.New("volume access type Block is unsupported")
	}

	if req.GetVolumeCapability().GetMount() == nil {
		return errors.New("volume access type must by Mount")
	}

	if req.GetStagingTargetPath() == "" {
		return errors.New("volume staging target path missing in request")
	}

	if req.GetVolumeCapability().GetAccessMode().GetMode() !=
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
		return fmt.Errorf("volume access mode must be ReadOnlyMany")
	}

	if volCtx := req.GetVolumeContext(); len(volCtx) > 0 {
		unsupportedVolumeParams := []string{"hash", "tag"}

		for _, volParam := range unsupportedVolumeParams {
			if _, ok := volCtx[volParam]; ok {
				return fmt.Errorf("volume parameter %s is not supported", volParam)
			}
		}
	}

	return nil
}

func validateNodeUnstageVolumeRequest(req *csi.NodeUnstageVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetStagingTargetPath() == "" {
		return errors.New("staging target path missing in request")
	}

	return nil
}
