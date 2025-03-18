package vantara

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/joho/godotenv"
	// "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

const decode = true

// Action types
const (
	GETLDEV        = "getLdev"
	ADDPATH        = "addPath"
	DELETEPATH     = "deletePath"
	GETPORTDETAILS = "getPortDetails"
)

type LUN struct {
	//Name is the volume name or just name in the storage system
	Name string
	// SerialNumber is a representation of the disk. With combination of the
	// vendor ID it should ve globally unique and can be identified by udev, usually
	// under /dev/disk/by-id/ with some prefix or postfix, depending on the udev rule
	// and can also be found by lsblk -o name,serial
	SerialNumber string
	// target's IQN
	IQN string
	// Storage provider ID in hex
	ProviderID string
	// the volume handle as set by the CSI driver field spec.volumeHandle
	VolumeHandle string
	//  Logical device ID of the volume
	LDeviceID string
	// Storage device Serial Number
	StorageSerialNumber string
	// Storage Protocol
	Protocol string
}

type V struct {
	Hostname string
	Username string
	Password string
}

func NewVantaraClonner(hostname, username, password string) (V, error) {
	return V{}, nil
}

func GetStorageEnvVars() (map[string]interface{}, error) {
	_ = godotenv.Load()
	envWWNs := os.Getenv("ESX_WWN_LIST")
	WWNs := []string{}
	if envWWNs != "" {
		items := strings.Split(envWWNs, ",")
		for _, item := range items {
			wwn := strings.TrimSpace(item)
			if wwn != "" {
				WWNs = append(WWNs, wwn)
			}
		}
	}
	envHGs := os.Getenv("HOSTGROUP_ID_LIST")
	HGs := []string{}
	if envHGs != "" {
		items := strings.Split(envHGs, ",")
		for _, item := range items {
			hg := strings.TrimSpace(item)
			if hg != "" {
				HGs = append(HGs, hg)
			}
		}
	}
	storageEnvVars := map[string]interface{}{
		"storageId":    os.Getenv("STORAGE_ID"),
		"restServerIP": os.Getenv("STORAGE_URL"),
		"port":         os.Getenv("STORAGE_PORT"),
		"userID":       os.Getenv("STORAGE_USERNAME"),
		"password":     os.Getenv("STORAGE_PASSWORD"),
		"HostWWN":      WWNs,
		"HostGroupID":  HGs,
		"LdevID":       os.Getenv("LDEV_ID"),
	}
	klog.Infof(
		"storageId: ", storageEnvVars["storageId"],
		"restServerIP: ", storageEnvVars["restServerIP"],
		"port: ", storageEnvVars["port"],
		"userID: ", "",
		"password: ", "",
		"HostWWN: ", storageEnvVars["HostWWN"],
		"HostGroupID: ", storageEnvVars["HostGroupID"],
		"LdevID: ", storageEnvVars["LdevID"],
	)
	return storageEnvVars, nil
}

func getNewVantaraStorageAPIfromEnv(envVars map[string]interface{}, vantaraObj VantaraObject) *VantaraStorageAPI {
	return NewVantaraStorageAPI(envVars["storageId"].(string), envVars["restServerIP"].(string), envVars["port"].(string), envVars["userID"].(string), envVars["password"].(string), vantaraObj)
}

func CurrentMappedGroups(targetLUN LUN) ([]string, error) {
	envStorage, _ := GetStorageEnvVars()
	return envStorage["HostGroupID"].([]string), nil
}

func ResolveVolumeHandleToLUN(volumeHandle string) (LUN, error) {
	parts := strings.Split(volumeHandle, "--")
	lun := LUN{}
	if len(parts) != 5 || parts[0] != "01" {
		return lun, fmt.Errorf("invalid volume handle: %s", volumeHandle)
	}
	ioProtocol := parts[1]
	storageDeviceID := parts[2]
	ldevID := parts[3]
	ldevNickName := parts[4]
	//storageModelID := storageDeviceID[:6]
	//storageSerialNumber := storageDeviceID[6:]

	lun.LDeviceID = ldevID
	//	LDEV := ShowLdev(lun)
	//	ldevnaaid := LDEV["naaId"].(string)
	lun.StorageSerialNumber = storageDeviceID
	lun.Protocol = ioProtocol
	//	lun.ProviderID = ldevnaaid[:6]
	//	lun.SerialNumber = ldevnaaid[6:]
	lun.VolumeHandle = volumeHandle
	lun.Name = ldevNickName
	return lun, nil
}

func GetNaaID(lun LUN) LUN {
	LDEV := ShowLdev(lun)
	ldevnaaid := LDEV["naaId"].(string)
	lun.ProviderID = ldevnaaid[:6]
	lun.SerialNumber = ldevnaaid[6:]
	return lun
}

func EnsureClonnerIgroup(xcopyInitiatorGroup []string, esxIQN []string) ([]string, error) {
	var r map[string]interface{}
	vantaraObj := make(VantaraObject)
	vantaraObj["initiatorGroup"] = xcopyInitiatorGroup
	envStorage, _ := GetStorageEnvVars()

	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)
	r, _ = v.VantaraStorage(GETPORTDETAILS)

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		fmt.Println("Error marshalling map to JSON:", err)
		return nil, err
	}

	var jsonData JSONData
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil, err
	}
	ret := FindHostGroupIDs(jsonData, envStorage["HostWWN"].([]string))

	jsonBytes, _ = json.MarshalIndent(ret, "", "  ")
	fmt.Println(string(jsonBytes))

	var hostGroupIds = make([]string, len(ret))
	for i, login := range ret {
		hostGroupIds[i] = login.HostGroupId
	}
	fmt.Println(hostGroupIds)
	return hostGroupIds, nil
}

func Map(xcopyInitiatorGroup []string, lun LUN) error {
	vantaraObj := make(VantaraObject)
	vantaraObj["ldevId"] = lun.LDeviceID
	vantaraObj["hostGroupIds"] = xcopyInitiatorGroup
	envStorage, _ := GetStorageEnvVars()

	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)
	_, _ = v.VantaraStorage(ADDPATH)
	return nil
}

func UnMap(xcopyInitiatorGroup []string, lun LUN) error {
	vantaraObj := make(VantaraObject)
	vantaraObj["ldevId"] = lun.LDeviceID
	vantaraObj["hostGroupIds"] = xcopyInitiatorGroup
	envStorage, _ := GetStorageEnvVars()

	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)
	_, _ = v.VantaraStorage(DELETEPATH)
	return nil
}

func ShowLdev(lun LUN) map[string]interface{} {
	vantaraObj := make(VantaraObject)
	vantaraObj["ldevId"] = lun.LDeviceID
	envStorage, _ := GetStorageEnvVars()

	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)
	r, _ := v.VantaraStorage(GETLDEV)
	return r
}
