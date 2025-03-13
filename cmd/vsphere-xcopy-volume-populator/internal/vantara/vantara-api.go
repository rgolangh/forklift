package vantara

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type storageDeviceIDPrefix string

const (
	STORAGE_DEVICEID_VSP_G1000_G1500_F1500             storageDeviceIDPrefix = "800000" // Zeus
	STORAGE_DEVICEID_VSP_G100_G200                     storageDeviceIDPrefix = "832000" // Panama
	STORAGE_DEVICEID_VSP_G400_F400_G600_F600           storageDeviceIDPrefix = "834000" // Panama
	STORAGE_DEVICEID_VSP_G800_F800                     storageDeviceIDPrefix = "836000" // Panama
	STORAGE_DEVICEID_VSP_G130                          storageDeviceIDPrefix = "880000" // Panama2
	STORAGE_DEVICEID_VSP_G150_G350_F350                storageDeviceIDPrefix = "882000" // Panama2
	STORAGE_DEVICEID_VSP_G370_F370_G700_F700_G900_F900 storageDeviceIDPrefix = "886000" // Panama2
	STORAGE_DEVICEID_VSP_5100_5200_5500_5600           storageDeviceIDPrefix = "900000" // Zeus2, Pre-Pegasus Ent
	STORAGE_DEVICEID_VSP_E990                          storageDeviceIDPrefix = "936000" // Panama2a
	STORAGE_DEVICEID_VSP_E390_E790_E590                storageDeviceIDPrefix = "934000" // Panama2a
	STORAGE_DEVICEID_VSP_E1090                         storageDeviceIDPrefix = "938000" // Pre-Pegasus Mid
	STORAGE_DEVICEID_VSP_ONE_B20                       storageDeviceIDPrefix = "A34000" // Pegasus Mid
)

var (
	STORAGE_DEVICE_MAP = map[storageDeviceIDPrefix]int{
		STORAGE_DEVICEID_VSP_G1000_G1500_F1500:             7,  //"VSP G1000/G1500/F1500",
		STORAGE_DEVICEID_VSP_G100_G200:                     12, //"VSP G100/G200",
		STORAGE_DEVICEID_VSP_G400_F400_G600_F600:           12, //"VSP G400/F400/G600/F600",
		STORAGE_DEVICEID_VSP_G800_F800:                     12, //"VSP G800/F800",
		STORAGE_DEVICEID_VSP_G130:                          12, //"VSP G130",
		STORAGE_DEVICEID_VSP_G150_G350_F350:                12, //"VSP G150/G350/F350",
		STORAGE_DEVICEID_VSP_G370_F370_G700_F700_G900_F900: 12, //"VSP G370/F370/G700/F700/G900/F900",
		STORAGE_DEVICEID_VSP_5100_5200_5500_5600:           8,  //"VSP 5100/5200/5500/5600",
		STORAGE_DEVICEID_VSP_E990:                          12, //"VSP E990",
		STORAGE_DEVICEID_VSP_E390_E790_E590:                21, //"VSP E390/E790/E590",
		STORAGE_DEVICEID_VSP_E1090:                         23, //"VSP E1090",
		STORAGE_DEVICEID_VSP_ONE_B20:                       28, //"VSP ONE B20",
	}
)

type BlockStorageAPI struct {
	GumIPAddr  string
	Port       string
	StorageID  string
	BaseURL    string
	ObjectURL  string
	ServiceURL string
}

func NewBlockStorageAPI(gumIPAddr, port, storageID string) *BlockStorageAPI {
	baseURL := fmt.Sprintf("https://%s:%s/ConfigurationManager/v1", gumIPAddr, port)
	return &BlockStorageAPI{
		GumIPAddr:  gumIPAddr,
		Port:       port,
		StorageID:  storageID,
		BaseURL:    baseURL,
		ObjectURL:  "/objects",
		ServiceURL: "/services",
	}
}

func (api *BlockStorageAPI) GetStorageID() string {
	return api.StorageID
}

func (api *BlockStorageAPI) Ldevs() string {
	return api.BaseURL + api.ObjectURL + "/ldevs"
}

func (api *BlockStorageAPI) Ldev(objectID string) string {
	return api.Ldevs() + "/" + objectID
}

func (api *BlockStorageAPI) GenerateSession() string {
	return api.BaseURL + api.ObjectURL + "/sessions"
}

func (api *BlockStorageAPI) DiscardSession(objectID string) string {
	return api.BaseURL + api.ObjectURL + "/sessions/" + objectID
}

func (api *BlockStorageAPI) Job(objectID string) string {
	return api.BaseURL + api.ObjectURL + "/jobs/" + objectID
}

func (api *BlockStorageAPI) Ports() string {
	return api.BaseURL + api.ObjectURL + "/ports"
}
func (api *BlockStorageAPI) Luns() string {
	return api.BaseURL + api.ObjectURL + "/luns"
}

func (api *BlockStorageAPI) Lun(objectID string) string {
	return api.Luns() + "/" + objectID
}

func (api *BlockStorageAPI) APIVersion() string {
	return fmt.Sprintf("https://%s:%s/ConfigurationManager/configuration/version", api.GumIPAddr, api.Port)
}

func MakeHTTPRequest(methodType, url string, body, headers map[string]string, authType, authValue string) (map[string]interface{}, error) {
	fmt.Println("Making HTTP request:")
	fmt.Println("Method:", methodType)
	fmt.Println("URL:", url)
	fmt.Println("Headers:", headers)
	fmt.Println("Auth Type:", authType)

	// Disable TLS certificate verification
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Create request body
	var reqBody io.Reader
	if body != nil { // If body is not nil, encode it as JSON
		jsonBody, err := json.Marshal(body)
		if err != nil {
			fmt.Println("Error encoding JSON:", err)
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(methodType, url, reqBody)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set authentication
	if authType == "basic" {
		// authValue should be "username:password"
		base64Auth := base64.StdEncoding.EncodeToString([]byte(authValue))
		req.Header.Set("Authorization", "Basic "+base64Auth)
	} else if authType == "session" {
		// authValue should be the token
		req.Header.Set("Authorization", authValue)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	fmt.Println("Response status:", resp.Status)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		fmt.Println("Request failed with status code:", resp.StatusCode)
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (api *BlockStorageAPI) checkUpdate(jobID string, headers map[string]string) (map[string]interface{}, error) {
	url := api.Job(jobID)
	return MakeHTTPRequest("GET", url, nil, headers, "", "")

}

func CheckAPIVersion(apiVersion string, requiredMajorVersion, requiredMinorVersion int) error {
	versionParts := strings.Split(apiVersion, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("invalid API version format: %s", apiVersion)
	}

	majorVersion, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("invalid major version: %s", versionParts[0])
	}

	minorVersion, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return fmt.Errorf("invalid minor version: %s", versionParts[1])
	}

	if !((majorVersion == requiredMajorVersion && minorVersion >= requiredMinorVersion) || majorVersion >= requiredMajorVersion+1) {
		return fmt.Errorf("this program requires API Version %d.%d.x or newer", requiredMajorVersion, requiredMinorVersion)
	}

	return nil
}

func (api *BlockStorageAPI) InvokeAsyncCommand(methodType, url string, body, headers map[string]string) (string, error) {

	result, err := MakeHTTPRequest(methodType, url, body, headers, "session", headers["Authorization"])
	if err != nil {
		return "", err
	}
	fmt.Println("Request was accepted. JOB URL:", result["self"])

	status := "Initializing"
	retryCount := 1
	waitTime := 1 // FIRST_WAIT_TIME

	for status != "Completed" {
		if retryCount > 10 { // MAX_RETRY_COUNT
			fmt.Println("Timeout error: operation was not completed")
			return "", fmt.Errorf("timeout error: operation was not completed")
		}
		time.Sleep(time.Duration(waitTime) * time.Second)

		jobResult, err := api.checkUpdate(fmt.Sprintf("%d", int(result["jobId"].(float64))), headers)
		if err != nil {
			fmt.Println("Error checking job status:", err)
			return "", err
		}
		status = jobResult["status"].(string)
		fmt.Println("Status:", status)
		if waitTime*2 < 120 {
			waitTime *= 2
		} else {
			waitTime = 120
		}
		retryCount++
	}

	fmt.Println("Async job was succeeded. status:", status)
	return status, nil
}
