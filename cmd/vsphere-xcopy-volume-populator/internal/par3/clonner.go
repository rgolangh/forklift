package par3

import (
	"context"
	"fmt"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

// const XCOPY_CLONNER_GROUP = "xcopy-service-vms"

type Par3Clonner struct {
	client Par3Client
}

func NewPar3Clonner(storageHostname, storageUsername, storagePassword string) (Par3Clonner, error) {
	clon := NewPar3ClientWsImpl(storageHostname, storageUsername, storagePassword)
	return Par3Clonner{
		client: &clon,
	}, nil
}

// EnsureClonnerIgroup creates or update an initiator group with the clonnerIqn
func (c *Par3Clonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn string) error {
	err := c.client.EnsureHostWithIqn(initiatorGroup, clonnerIqn)
	if err != nil {
		return fmt.Errorf("failed to ensure host with IQN: %w", err)
	}

	err = c.client.EnsureHostSetExists(initiatorGroup)
	if err != nil {
		return fmt.Errorf("failed to ensure host set: %w", err)
	}

	err = c.client.AddHostToHostSet(initiatorGroup, initiatorGroup)
	if err != nil {
		return fmt.Errorf("failed to add host to host set: %w", err)
	}

	return nil
}

// Map is responsible to mapping an initiator group to a LUN
func (c *Par3Clonner) Map(initiatorGroup string, targetLUN populator.LUN) error {
	return c.client.EnsureLunMapped(initiatorGroup, targetLUN)
}

// UnMap is responsible to unmapping an initiator group from a LUN
func (c *Par3Clonner) UnMap(initiatorGroup string, targetLUN populator.LUN) error {
	return c.client.LunUnmap(context.TODO(), initiatorGroup, targetLUN.Name)
}

// Return initiatorGroups the LUN is mapped to
func (c *Par3Clonner) CurrentMappedGroups(targetLUN populator.LUN) ([]string, error) {
	return []string{}, fmt.Errorf("Par3Clonner currentMappedGroups not implemented yet")
}

func (c *Par3Clonner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	serialNumber, err := c.client.GetLunSerialNumber(volumeHandle)
	if err != nil {
		return populator.LUN{}, err
	}
	fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>><<<<<<<")
	fmt.Println(serialNumber)

	// in RHEL lsblk needs that swap. In fedora it doesn't
	//serialNumber :=  strings.ReplaceAll(l.SerialNumber, "?", "\\\\x3f")
	lun := populator.LUN{Name: volumeHandle, VolumeHandle: volumeHandle, SerialNumber: serialNumber, ProviderID: "60002ac"}
	return lun, nil

	//return populator.LUN{}, fmt.Errorf("Par3Clonner ResolveVolumeHandle not implemented yet")
}
