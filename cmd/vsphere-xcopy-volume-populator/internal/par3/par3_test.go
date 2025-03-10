package par3

import (
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/stretchr/testify/assert"
)

func TestPar3Clonner(t *testing.T) {
	mockClient := NewMockPar3Client()
	clonner := Par3Clonner{client: mockClient}

	targetLUN := populator.LUN{
		Name:         "TestVolume",
		SerialNumber: "123456789ABC",
		IQN:          "iqn.1993-08.org.debian:01:test1234",
	}
	initiatorGroup := "TestInitiatorGroup"

	t.Run("Map LUN", func(t *testing.T) {
		err := clonner.Map(initiatorGroup, &targetLUN)
		assert.NoError(t, err, "Expected no error when mapping LUN")

		lunID, err := mockClient.GetVLunID(targetLUN.Name, initiatorGroup)
		assert.NoError(t, err, "Expected to retrieve LUN ID")
		assert.Greater(t, lunID, 0, "Expected a valid LUN ID")
		fmt.Printf("Mapped LUN ID: %d\n", lunID)
	})

	t.Run("Unmap LUN", func(t *testing.T) {
		err := clonner.Map(initiatorGroup, &targetLUN)
		assert.NoError(t, err, "Expected no error when mapping LUN")

		err = clonner.UnMap(initiatorGroup, targetLUN)
		assert.NoError(t, err, "Expected no error when unmapping LUN")

		_, err = mockClient.GetVLunID(targetLUN.Name, initiatorGroup)
		assert.Error(t, err, "Expected an error because LUN should be unmapped")
	})

	t.Run("Ensure Clonner Igroup", func(t *testing.T) {
		hostName := "TestHost"
		iqn := "iqn.1993-08.org.debian:01:test1234"

		err := clonner.EnsureClonnerIgroup(hostName, iqn)
		assert.NoError(t, err, "Expected no error when ensuring Clonner Igroup")

		_, hostExists := mockClient.Hosts[hostName]
		assert.True(t, hostExists, "Expected host to exist")

		_, hostSetExists := mockClient.HostSets[initiatorGroup]
		assert.True(t, hostSetExists, "Expected host set to exist")
	})

	t.Run("Ensure Host with IQN", func(t *testing.T) {
		hostName := "Host1"
		iqn := "iqn.1993-08.org.debian:01:host1"

		_, err := mockClient.EnsureHostWithIqn(iqn)
		assert.NoError(t, err, "Expected no error when ensuring host with IQN")
		assert.Equal(t, iqn, mockClient.Hosts[hostName], "Expected host to have correct IQN")
	})

	t.Run("Add Host to Host Set", func(t *testing.T) {
		hostSetName := "HostSet1"
		hostName := "Host1"

		err := mockClient.EnsureHostSetExists(hostSetName)
		assert.NoError(t, err, "Expected no error when creating host set")

		err = mockClient.AddHostToHostSet(hostSetName, hostName)
		assert.NoError(t, err, "Expected no error when adding host to host set")
		assert.Contains(t, mockClient.HostSets[hostSetName], hostName, "Expected host to be in host set")
	})
}
