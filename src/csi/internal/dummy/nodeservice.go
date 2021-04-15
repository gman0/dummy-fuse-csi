package dummy

import (
	"context"
	"fmt"
	"sync"

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

func (ns *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if err := validateNodePublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, isPending := ns.pendingVolOpts.LoadOrStore(req.GetVolumeId(), true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", req.GetVolumeId()))
	}
	defer ns.pendingVolOpts.Delete(req.GetVolumeId())

	targetPath := req.GetTargetPath()

	if err := makeMountpoint(targetPath); err != nil {
		// Failed to mkdir
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("failed to create mountpoint for volume %s at %v: %v",
				req.GetVolumeId(), targetPath, err))
	}

	if mounted, err := isMountpoint(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if mounted {
		// Already mounted
		return &csi.NodePublishVolumeResponse{}, nil
	}

	if err := bindMount(req.GetStagingTargetPath(), targetPath); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cachePublishMount(req.GetVolumeId(), req.GetStagingTargetPath(), req.GetTargetPath(), ns.d.DriverOpts.MountCachePath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := validateNodeUnpublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, isPending := ns.pendingVolOpts.LoadOrStore(req.GetVolumeId(), true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", req.GetVolumeId()))
	}
	defer ns.pendingVolOpts.Delete(req.GetVolumeId())

	targetPath := req.GetTargetPath()

	if exists, err := pathExists(targetPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to stat %s for volume %s: %v", targetPath, req.GetVolumeId(), err.Error()))
	} else if !exists {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	if mounted, err := isMountpoint(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if mounted {
		if err = bindUnmount(targetPath); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmount bind %s for volume %s: %v", targetPath, req.GetVolumeId(), err))
		}
	}

	if err := rmMountpoint(targetPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to remove mountpoint %s for volume %s: %v", targetPath, req.GetVolumeId(), err))
	}

	forgetPublishMount(req.GetVolumeId(), ns.d.DriverOpts.MountCachePath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if err := validateNodeStageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, isPending := ns.pendingVolOpts.LoadOrStore(req.GetVolumeId(), true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", req.GetVolumeId()))
	}
	defer ns.pendingVolOpts.Delete(req.GetVolumeId())

	stagingTargetPath := req.GetStagingTargetPath()

	if mounted, err := isMountpoint(stagingTargetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if mounted {
		// Already mounted
		return &csi.NodeStageVolumeResponse{}, nil
	}

	if err := fuseMount(stagingTargetPath); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cacheStageMount(req.GetVolumeId(), req.GetStagingTargetPath(), ns.d.DriverOpts.MountCachePath)

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if err := validateNodeUnstageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, isPending := ns.pendingVolOpts.LoadOrStore(req.GetVolumeId(), true); isPending {
		// CO should try again
		return nil, status.Error(codes.Aborted, fmt.Sprintf("volume %s is already being processed", req.GetVolumeId()))
	}
	defer ns.pendingVolOpts.Delete(req.GetVolumeId())

	stagingTargetPath := req.GetStagingTargetPath()

	if exists, err := pathExists(stagingTargetPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to stat %s for volume %s: %v", stagingTargetPath, req.GetVolumeId(), err.Error()))
	} else if !exists {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if mounted, err := isMountpoint(stagingTargetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if mounted {
		if err = fuseUnmount(stagingTargetPath); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmount %s for volume %s: %v", stagingTargetPath, req.GetVolumeId(), err))
		}
	}

	forgetStageMount(req.GetVolumeId(), ns.d.DriverOpts.MountCachePath)

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RPC not implemented")
}

func (ns *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RPC not implemented")
}
