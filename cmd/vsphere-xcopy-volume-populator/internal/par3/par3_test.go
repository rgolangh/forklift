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
		err := clonner.Map(initiatorGroup, targetLUN)
		assert.NoError(t, err, "Expected no error when mapping LUN")

		lunID, err := mockClient.GetLunID(targetLUN.Name, initiatorGroup)
		assert.NoError(t, err, "Expected to retrieve LUN ID")
		assert.Greater(t, lunID, 0, "Expected a valid LUN ID")
		fmt.Printf("Mapped LUN ID: %d\n", lunID)
	})

	t.Run("Unmap LUN", func(t *testing.T) {
		err := clonner.UnMap(initiatorGroup, targetLUN)
		assert.NoError(t, err, "Expected no error when unmapping LUN")

		_, err = mockClient.GetLunID(targetLUN.Name, initiatorGroup)
		assert.Error(t, err, "Expected an error because LUN should be unmapped")
	})

	t.Run("EnsureClonnerIgroup", func(t *testing.T) {
		mockClient := NewMockPar3Client()
		clonner := &Par3Clonner{client: mockClient}
		hostName := "TestHost"
		iqn := "iqn.1993-08.org.debian:01:test1234"

		t.Run("Ensure Clonner Igroup", func(t *testing.T) {
			err := clonner.EnsureClonnerIgroup(hostName, iqn)
			assert.NoError(t, err, "Expected no error when ensuring Clonner Igroup")

			_, hostExists := mockClient.Hosts[hostName]
			assert.True(t, hostExists, "Expected host to exist")

			_, hostSetExists := mockClient.Hosts[hostName]
			assert.True(t, hostSetExists, "Expected host set to exist")
		})
	})
}
