package par3

import (
	"context"
	"fmt"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"log"
	"sync"
)

type MockPar3Client struct {
	SessionKey  string
	LUNMappings map[string]map[string]int
	UsedLUNs    map[string]map[int]bool
	Hosts       map[string]string
	HostSets    map[string][]string
	Mutex       sync.Mutex
}

func NewMockPar3Client() *MockPar3Client {
	return &MockPar3Client{
		SessionKey:  "mock-session-key",
		LUNMappings: make(map[string]map[string]int),
		UsedLUNs:    make(map[string]map[int]bool),
		Hosts:       make(map[string]string),
		HostSets:    make(map[string][]string),
	}
}
func (m *MockPar3Client) GetLunDetailsByVolumeName(volumeName string, lun *populator.LUN) error {
	return nil
}

func (m *MockPar3Client) CurrentMappedGroups(volumeName string) ([]string, error) {
	return []string{}, nil
}

func (m *MockPar3Client) GetSessionKey() (string, error) {
	log.Println("Mock: GetSessionKey called")
	m.SessionKey = "mock-session-key"
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

	log.Printf("Mock: EnsureLunMapped -> Volume %s mapped to initiator group %s with LUN ID %d", targetLUN.Name, initiatorGroup, lunID)
	return nil
}

func (m *MockPar3Client) GetLunByVolumeName(initiatorGroup string) (string, error) {
	return "", nil
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
	return "hostname", m.createHost("hostname", iqn)
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

func (m *MockPar3Client) hostExists(hostname string) (bool, error) {
	if _, exists := m.Hosts[hostname]; exists {
		return true, nil
	}
	return false, nil
}

func (m *MockPar3Client) createHost(hostname, iqn string) error {
	m.Hosts[hostname] = iqn
	log.Printf("Mock: Created host %s with IQN %s", hostname, iqn)
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

func (m *MockPar3Client) GetLunID(lunName, initiatorGroupName string) (int, error) {
	if lunID, exists := m.LUNMappings[initiatorGroupName][lunName]; exists {
		return lunID, nil
	}

	return 0, fmt.Errorf("mock: LUN ID not found for volume %s and initiator group %s", lunName, initiatorGroupName)
}
