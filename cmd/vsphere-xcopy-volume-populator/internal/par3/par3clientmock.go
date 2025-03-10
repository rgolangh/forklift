package par3

import (
	"context"
	"fmt"
	"log"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

type MockPar3Client struct {
	SessionKey  string
	LUNMappings map[string]map[string]int
	UsedLUNs    map[string]map[int]bool
	Hosts       map[string]string
	HostSets    map[string][]string
	Volumes     map[string]populator.LUN
}

func NewMockPar3Client() *MockPar3Client {
	return &MockPar3Client{
		SessionKey:  "mock-session-key",
		LUNMappings: make(map[string]map[string]int),
		UsedLUNs:    make(map[string]map[int]bool),
		Hosts:       make(map[string]string),
		HostSets:    make(map[string][]string),
		Volumes:     make(map[string]populator.LUN),
	}
}

func (m *MockPar3Client) GetSessionKey() (string, error) {
	log.Println("Mock: GetSessionKey called")
	return m.SessionKey, nil
}

func (m *MockPar3Client) EnsureLunMapped(initiatorGroup string, targetLUN *populator.LUN) error {
	lunID, err := m.GetFreeLunID(initiatorGroup)
	if err != nil {
		return err
	}

	if _, exists := m.LUNMappings[initiatorGroup]; !exists {
		m.LUNMappings[initiatorGroup] = make(map[string]int)
	}
	m.LUNMappings[initiatorGroup][targetLUN.Name] = lunID

	if _, exists := m.UsedLUNs[initiatorGroup]; !exists {
		m.UsedLUNs[initiatorGroup] = make(map[int]bool)
	}
	m.UsedLUNs[initiatorGroup][lunID] = true

	m.Volumes[targetLUN.Name] = *targetLUN

	log.Printf("Mock: EnsureLunMapped -> Volume %s mapped to initiator group %s with LUN ID %d", targetLUN.Name, initiatorGroup, lunID)
	return nil
}

func (m *MockPar3Client) LunUnmap(ctx context.Context, initiatorGroupName, lunName string) error {
	if lunID, exists := m.LUNMappings[initiatorGroupName][lunName]; exists {
		delete(m.LUNMappings[initiatorGroupName], lunName)
		delete(m.UsedLUNs[initiatorGroupName], lunID)

		log.Printf("Mock: LunUnmap -> Volume %s unmapped from initiator group %s (LUN ID: %d)", lunName, initiatorGroupName, lunID)
		return nil
	}

	return fmt.Errorf("mock: LUN %s not found for initiator group %s", lunName, initiatorGroupName)
}

func (m *MockPar3Client) EnsureHostWithIqn(iqn string) (string, error) {
	hostName := fmt.Sprintf("host-%s", iqn)
	m.Hosts[hostName] = iqn
	log.Printf("Mock: Created host %s with IQN %s", hostName, iqn)
	return hostName, nil
}

func (m *MockPar3Client) EnsureHostSetExists(hostSetName string) error {
	if _, exists := m.HostSets[hostSetName]; exists {
		return nil
	}

	m.HostSets[hostSetName] = []string{}
	log.Printf("Mock: Created host set %s", hostSetName)
	return nil
}

func (m *MockPar3Client) AddHostToHostSet(hostSetName string, hostName string) error {
	if _, exists := m.HostSets[hostSetName]; !exists {
		return fmt.Errorf("mock: host set %s does not exist", hostSetName)
	}

	if _, exists := m.Hosts[hostName]; !exists {
		return fmt.Errorf("mock: host %s does not exist", hostName)
	}

	for _, existingHost := range m.HostSets[hostSetName] {
		if existingHost == hostName {
			return nil
		}
	}

	m.HostSets[hostSetName] = append(m.HostSets[hostSetName], hostName)
	log.Printf("Mock: Added host %s to host set %s", hostName, hostSetName)
	return nil
}

func (m *MockPar3Client) GetFreeLunID(initiatorGroupName string) (int, error) {
	if _, exists := m.UsedLUNs[initiatorGroupName]; !exists {
		m.UsedLUNs[initiatorGroupName] = make(map[int]bool)
	}

	for i := 1; i <= 255; i++ {
		if !m.UsedLUNs[initiatorGroupName][i] {
			return i, nil
		}
	}

	return 0, fmt.Errorf("mock: no available LUN ID for initiator group %s", initiatorGroupName)
}

func (m *MockPar3Client) GetLunDetailsByVolumeName(lunName string, lun *populator.LUN) error {
	if volume, exists := m.Volumes[lunName]; exists {
		*lun = volume
		log.Printf("Mock: GetLunDetailsByVolumeName -> Found volume %s", lunName)
		return nil
	}

	return fmt.Errorf("mock: volume %s not found", lunName)
}

func (m *MockPar3Client) CurrentMappedGroups(volumeName string) ([]string, error) {
	var groups []string

	for group, mappings := range m.LUNMappings {
		if _, exists := mappings[volumeName]; exists {
			groups = append(groups, group)
		}
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("mock: no mapped groups found for volume %s", volumeName)
	}

	log.Printf("Mock: CurrentMappedGroups -> Volume %s is mapped to groups: %v", volumeName, groups)
	return groups, nil
}

func (m *MockPar3Client) GetVLunID(lunName, initiatorGroupName string) (int, error) {
	if lunID, exists := m.LUNMappings[initiatorGroupName][lunName]; exists {
		return lunID, nil
	}

	return 0, fmt.Errorf("mock: LUN ID not found for volume %s and initiator group %s", lunName, initiatorGroupName)
}
