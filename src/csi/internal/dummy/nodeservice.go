package dummy

import (
	"context"
	"fmt"
	"log"
	"sync"

	"dummy-fuse-csi/internal/dummy/mountutils"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type nodeService struct {
	d             *Driver
	caps          []*csi.NodeServiceCapability
	accessibility *csi.Topology

	pendingVolOpts sync.Map
}

func newNodeService(d *Driver) csi.NodeServer {
	supportedRpcs := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	}

	var caps []*csi.NodeServiceCapability
	for _, c := range supportedRpcs {
		caps = append(caps, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: c,
				},
			},
		})
	}

	return &nodeService{
		d:    d,
		caps: caps,
	}
}

func (ns *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.caps,
	}, nil
}

func (ns *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.d.NodeID,
	}, nil
}

func tryFuseMount(mountpoint string) error {
	return tryMount("", mountpoint, fuseMounter{})
}

func tryBindMount(bindFrom, bindTo string) error {
	return tryMount(bindFrom, bindTo, bindMounter{})
}

func tryMount(dev, mountpoint string, m mounterUnmounter) error {
	mntState, err := mountutils.GetState(mountpoint)
	if err != nil {
		return err
	}

	switch mntState {
	case mountutils.StCorrupted:
		if err = m.unmount(mountpoint); err != nil {
			log.Printf("failed to unmount corrupted mount: %v", err)
		}
		fallthrough
	case mountutils.StNotMounted:
		if err = m.mount(dev, mountpoint); err != nil {
			log.Printf("failed to mount: %v", err)
		}
	case mountutils.StMounted:
		return nil
	default:
		return fmt.Errorf("bad mount state %s", mntState)
	}

	return nil
}

func (ns *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if err := validateNodePublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var (
		volID             = req.GetVolumeId()
		stagingTargetPath = req.GetStagingTargetPath()
		targetPath        = req.GetTargetPath()
	)

	if _, isPending := ns.pendingVolOpts.LoadOrStore(volID, true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", volID))
	}
	defer ns.pendingVolOpts.Delete(volID)

	if err := makeMountpoint(targetPath); err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("failed to create mountpoint for volume %s at %v: %v",
				volID, targetPath, err))
	}

	if err := tryFuseMount(stagingTargetPath); err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("failed to mount volume %s in staging target path %s: %v",
				volID, stagingTargetPath, err))
	}

	if err := tryBindMount(stagingTargetPath, targetPath); err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("failed to mount volume %s in target path %s: %v",
				volID, targetPath, err))
	}

	cachePublishMount(volID, stagingTargetPath, targetPath, ns.d.DriverOpts.MountCachePath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := validateNodeUnpublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var (
		volID      = req.GetVolumeId()
		targetPath = req.GetTargetPath()
	)

	if _, isPending := ns.pendingVolOpts.LoadOrStore(volID, true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", volID))
	}
	defer ns.pendingVolOpts.Delete(volID)

	mntState, err := mountutils.GetState(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get mount state for %s: %v", targetPath, err)
	}

	switch mntState {
	case mountutils.StUnknown:
		return nil, status.Errorf(codes.Internal, "unkown mount state for %s", targetPath)
	case mountutils.StMounted, mountutils.StCorrupted:
		if err := (bindMounter{}).unmount(targetPath); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmount bind %s for volume %s: %v", targetPath, volID, err))
		}
	}

	if err := rmMountpoint(targetPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to remove mountpoint %s for volume %s: %v", targetPath, volID, err))
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if err := validateNodeStageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var (
		volID             = req.GetVolumeId()
		stagingTargetPath = req.GetStagingTargetPath()
	)

	if _, isPending := ns.pendingVolOpts.LoadOrStore(volID, true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", volID))
	}
	defer ns.pendingVolOpts.Delete(volID)

	if err := tryFuseMount(stagingTargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount FUSE in %s: %v", stagingTargetPath, err)
	}

	cacheStageMount(volID, req.GetStagingTargetPath(), ns.d.DriverOpts.MountCachePath)

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if err := validateNodeUnstageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var (
		volID             = req.GetVolumeId()
		stagingTargetPath = req.GetStagingTargetPath()
	)

	if _, isPending := ns.pendingVolOpts.LoadOrStore(volID, true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", volID))
	}
	defer ns.pendingVolOpts.Delete(volID)

	mntState, err := mountutils.GetState(stagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get mount state for %s: %v", stagingTargetPath, err)
	}

	switch mntState {
	case mountutils.StUnknown:
		return nil, status.Errorf(codes.Internal, "unkown mount state for %s", stagingTargetPath)
	case mountutils.StMounted, mountutils.StCorrupted:
		if err := (fuseMounter{}).unmount(stagingTargetPath); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmount FUSE %s for volume %s: %v", stagingTargetPath, volID, err))
		}
	}

	forgetStageMount(volID, ns.d.DriverOpts.MountCachePath)

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RPC not implemented")
}

func (ns *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RPC not implemented")
}
