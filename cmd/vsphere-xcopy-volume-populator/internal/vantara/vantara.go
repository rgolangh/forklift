package vantara

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/joho/godotenv"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

const decode = true

// Action types
const (
	GETLDEV        = "getLdev"
	ADDPATH        = "addPath"
	DELETEPATH     = "deletePath"
	GETPORTDETAILS = "getPortDetails"
)

type VantaraCloner struct {
	api VantaraStorageAPI
}

func NewVantaraClonner(hostname, username, password string) (VantaraCloner, error) {
	vantaraObj := make(VantaraObject)
	envStorage, _ := getStorageEnvVars()
	vantaraObj["xcopyInitiatorGroup"] = envStorage["HostWWN"]
	vantaraObj["hostGroupIds"] = envStorage["HostGroupID"]
	vantaraObj["ldevId"] = envStorage["LdevID"]
	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)

	return VantaraCloner{api: *v}, nil
}

func getStorageEnvVars() (map[string]interface{}, error) {
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

func (v *VantaraCloner) CurrentMappedGroups(targetLUN populator.LUN) ([]string, error) {
	return v.api.VantaraObj["hostGroupIds"].([]string), nil
}

func (v *VantaraCloner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	parts := strings.Split(volumeHandle, "--")
	lun := populator.LUN{}
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

func (v *VantaraCloner) GetNaaID(lun populator.LUN) populator.LUN {
	LDEV := v.ShowLdev(lun)
	ldevnaaid := LDEV["naaId"].(string)
	lun.ProviderID = ldevnaaid[:6]
	lun.SerialNumber = ldevnaaid[6:]
	return lun
}

func (v *VantaraCloner) EnsureClonnerIgroup(xcopyInitiatorGroup []string, esxIQN []string) ([]string, error) {
	var r map[string]interface{}

	r, _ = v.api.VantaraStorage(GETPORTDETAILS)

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
	ret := FindHostGroupIDs(jsonData, v.api.VantaraObj["xcopyInitiatorGroup"].([]string))

	jsonBytes, _ = json.MarshalIndent(ret, "", "  ")
	fmt.Println(string(jsonBytes))

	var hostGroupIds = make([]string, len(ret))
	for i, login := range ret {
		hostGroupIds[i] = login.HostGroupId
	}
	fmt.Println(hostGroupIds)
	return hostGroupIds, nil
}

func (v *VantaraCloner) Map(xcopyInitiatorGroup []string, lun populator.LUN) error {
	_, _ = v.api.VantaraStorage(ADDPATH)
	return nil
}

func (v *VantaraCloner) UnMap(xcopyInitiatorGroup []string, lun populator.LUN) error {
	_, _ = v.api.VantaraStorage(DELETEPATH)
	return nil
}

func (v *VantaraCloner) ShowLdev(lun populator.LUN) map[string]interface{} {
	r, _ := v.api.VantaraStorage(GETLDEV)
	return r
}
