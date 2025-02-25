package par3

import (
	"context"
	"fmt"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

const XCOPY_CLONNER_GROUP = "xcopy-service-vms"

type Par3Clonner struct {
	client Par3Client
}

// EnsureClonnerIgroup creates or update an initiator group with the clonnerIqn
func (c *Par3Clonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn string) error {
	return fmt.Errorf("par3Clonner not yet implemented")
}

// Map is responsible to mapping an initiator group to a LUN
func (c *Par3Clonner) Map(initiatorGroup string, targetLUN populator.LUN) error {
	return c.client.EnsureLunMapped(initiatorGroup, targetLUN)
}

// UnMap is responsible to unmapping an initiator group from a LUN
func (c *Par3Clonner) UnMap(initiatorGroup string, targetLUN populator.LUN) error {
	return c.client.LunUnmap(context.TODO(), initiatorGroup, fmt.Sprintf("/vol/%s/lun0", targetLUN.Name))
}

// Return initiatorGroups the LUN is mapped to
func (c *Par3Clonner) CurrentMappedGroups(targetLUN populator.LUN) ([]string, error) {
	return []string{}, fmt.Errorf("Par3Clonner currentMappedGroups not implemented yet")
}
