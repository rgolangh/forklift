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
	Mutex       sync.Mutex
}

func NewMockPar3Client() *MockPar3Client {
	return &MockPar3Client{
		SessionKey:  "mock-session-key",
		LUNMappings: make(map[string]map[string]int),
		UsedLUNs:    make(map[string]map[int]bool),
		Hosts:       make(map[string]string),
	}
}

func (m *MockPar3Client) GetSessionKey() (string, error) {
	log.Println("Mock: GetSessionKey called")
	m.SessionKey = "mock-session-key"
	return m.SessionKey, nil
}

func (m *MockPar3Client) EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

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

func (m *MockPar3Client) LunUnmap(ctx context.Context, initiatorGroupName, lunName string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	if lunID, exists := m.LUNMappings[initiatorGroupName][lunName]; exists {
		delete(m.LUNMappings[initiatorGroupName], lunName)
		delete(m.UsedLUNs[initiatorGroupName], lunID)

		log.Printf("Mock: LunUnmap -> Volume %s unmapped from initiator group %s (LUN ID: %d)", lunName, initiatorGroupName, lunID)
		return nil
	}

	return fmt.Errorf("mock: LUN %s not found for initiator group %s", lunName, initiatorGroupName)
}

func (m *MockPar3Client) EnsureHostWithIqn(initiatorGroupName string, iqn string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	if exists, _ := m.hostExists(initiatorGroupName); exists {
		return nil
	}

	return m.createHost(initiatorGroupName, iqn)
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
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

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
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	if lunID, exists := m.LUNMappings[initiatorGroupName][lunName]; exists {
		return lunID, nil
	}

	return 0, fmt.Errorf("mock: LUN ID not found for volume %s and initiator group %s", lunName, initiatorGroupName)
}
