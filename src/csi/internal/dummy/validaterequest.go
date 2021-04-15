package dummy

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func validateCreateVolumeRequest(req *csi.CreateVolumeRequest) error {
	if req.GetName() == "" {
		return errors.New("volume name cannot be empty")
	}

	reqCaps := req.GetVolumeCapabilities()
	if reqCaps == nil {
		return errors.New("volume capabilities cannot be empty")
	}

	for _, volCap := range reqCaps {
		if volCap.GetBlock() != nil {
			return errors.New("block access type not allowed")
		}
	}

	return nil
}

func validateDeleteVolumeRequest(req *csi.DeleteVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID cannot be empty")
	}

	return nil
}

func validateValidateVolumeCapabilitiesRequest(req *csi.ValidateVolumeCapabilitiesRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return errors.New("volume capabilities cannot be nil or empty")
	}

	return nil
}

func validateCreateSnapshotRequest(req *csi.CreateSnapshotRequest) error {
	if req.GetName() == "" {
		return errors.New("snapshot name cannot be empty")
	}

	if req.GetSourceVolumeId() == "" {
		return errors.New("source volume ID cannot be empty")
	}

	return nil
}

func validateDeleteSnapshotRequest(req *csi.DeleteSnapshotRequest) error {
	if req.GetSnapshotId() == "" {
		return errors.New("snapshot ID cannot be empty")
	}

	return nil
}

func validateNodePublishVolumeRequest(req *csi.NodePublishVolumeRequest) error {
	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}

	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	return nil
}

func validateNodeUnpublishVolumeRequest(req *csi.NodeUnpublishVolumeRequest) error {
	if req.GetTargetPath() == "" {
		return errors.New("target path missing in request")
	}

	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	return nil
}

func validateNodeStageVolumeRequest(req *csi.NodeStageVolumeRequest) error {
	if req.GetStagingTargetPath() == "" {
		return errors.New("staging path missing in request")
	}

	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}

	return nil
}

func validateNodeUnstageVolumeRequest(req *csi.NodeUnstageVolumeRequest) error {
	if req.GetStagingTargetPath() == "" {
		return errors.New("staging path missing in request")
	}

	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}

	return nil
}

func validateVolumeIsCompatible(existingVolume *csi.Volume, newVolume *csi.Volume) error {
	if existingVolume.GetCapacityBytes() != newVolume.GetCapacityBytes() {
		return errors.New("mismatch in size")
	}

	existingSrc := existingVolume.GetContentSource()
	newSrc := newVolume.GetContentSource()

	if existingSrc == nil && newSrc != nil {
		return errors.New("new volume is has a content source, but the existing one doesn't")
	}

	if newSrc == nil && existingSrc != nil {
		return errors.New("existing volume is has a content source, but the new one doesn't")
	}

	if existingSrc != nil && newSrc != nil {
		if !(existingSrc.GetSnapshot() != nil && newSrc.GetSnapshot() != nil || existingSrc.GetSnapshot() != nil) {
			return errors.New("mismatch in volume source")
		}
		if newSrc.GetSnapshot() == nil && newSrc.GetVolume() == nil {
			return errors.New("unsupported volume content type")
		}
	}

	return nil
}

func validateSnapshotIsCompatible(existingSnapshot *csi.Snapshot, newSnapshot *csi.Snapshot) error {
	if existingSnapshot.GetSnapshotId() != newSnapshot.GetSnapshotId() {
		return errors.New("mismatch in name")
	}

	if existingSnapshot.GetSourceVolumeId() != newSnapshot.GetSourceVolumeId() {
		return errors.New("mismatch in source volume ID")
	}

	return nil
}
