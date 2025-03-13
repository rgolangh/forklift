package vantara

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const requiredMajorVersion = 1
const requiredMinorVersion = 9

type VantaraStorageAPI struct {
	StorageID    string
	RestServerIP string
	RestSvrPort  string
	UserID       string
	Password     string
	VanataObj    VantaraObject
}

type VantaraObject map[string]interface {
}

func NewVantaraStorageAPI(storageID, restServerIP, restSvrPort, userID, password string, vantaraObj VantaraObject) *VantaraStorageAPI {
	return &VantaraStorageAPI{
		StorageID:    storageID,
		RestServerIP: restServerIP,
		RestSvrPort:  restSvrPort,
		UserID:       userID,
		Password:     password,
		VanataObj:    vantaraObj,
	}
}

// This function decodes a base64-encoded string and returns the decoded value.
func decodeBase64(encoded string) string {
	return encoded
	// decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	//
	//	if err != nil {
	//		panic(err)
	//	}
	//
	// return string(decodedBytes)
}

func extractIPAddress(url string) (string, error) {
	ipRegex := `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`
	r := regexp.MustCompile(ipRegex)
	match := r.FindString(url)
	if match == "" {
		return "", errors.New("IP address not found")
	}
	return match, nil
}

type PortsEntry struct {
	HostGroupName   string  `json:"hostGroupName"`
	HostGroupNumber float64 `json:"hostGroupNumber"`
	Lun             float64 `json:"lun"`
	PortID          string  `json:"portId"`
}

type LdevEntry struct {
	Ports []PortsEntry `json:"ports"`
}

func getlunID(ldevJson LdevEntry, hostGroupId string) (string, error) {
	parts := strings.SplitN(hostGroupId, ",", 2)
	portID := parts[0]
	hostGroupNumber := parts[1]
	for _, port := range ldevJson.Ports {
		if port.PortID == portID && fmt.Sprintf("%d", int(port.HostGroupNumber)) == hostGroupNumber {
			return fmt.Sprintf("%d", int(port.Lun)), nil
		}
	}
	return "", errors.New("LUN not found")
}

func (v *VantaraStorageAPI) VantaraStorage(actionType string) (map[string]interface{}, error) {
	headers := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json",
		"Response-Job-Status": "Completed",
	}
	body := map[string]string{}
	sessionId := "0"
	var err error
	var decodedIp, userCreds string

	if decode != false {
		decodedIp, err = extractIPAddress(decodeBase64(v.RestServerIP))

		decodedUserID := decodeBase64(v.UserID)
		decodedPassword := decodeBase64(v.Password)
		userCreds = decodedUserID + ":" + decodedPassword
	} else {
		decodedIp, err = extractIPAddress(v.RestServerIP)
		userCreds = v.UserID + ":" + v.Password
	}

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	api := NewBlockStorageAPI(decodedIp, v.RestSvrPort, v.StorageID)

	println(api)

	// Check API version
	url := api.APIVersion()
	println(url)
	r, err := MakeHTTPRequest("GET", url, nil, headers, "basic", userCreds)
	if err != nil {
		fmt.Println("Failed to get API version")
		return nil, err
	}
	apiVersion := r["apiVersion"].(string)
	CheckAPIVersion(apiVersion, requiredMajorVersion, requiredMinorVersion)

	// Generate a session
	url = api.GenerateSession()
	r, err = MakeHTTPRequest("POST", url, body, headers, "basic", userCreds)
	if err != nil {
		fmt.Println("Failed to generate session")
		return nil, err
	}
	fmt.Println("Session generated successfully:", r)
	// Discard session after the function returns
	defer func() {
		url = api.DiscardSession(sessionId)
		resp, err := MakeHTTPRequest("DELETE", url, body, headers, "session", headers["Authorization"])
		if err != nil {
			fmt.Println("Failed to discard session:", err)
			return
		}
		fmt.Println("Session discarded successfully:", resp)
	}()

	token := r["token"].(string)
	auth := "Session " + token
	sessionIdFloat64 := r["sessionId"].(float64)
	sessionIdInt := int(sessionIdFloat64)
	sessionId = fmt.Sprintf("%d", sessionIdInt)
	headers["Authorization"] = auth

	switch actionType {
	case GETLDEV:
		url = api.Ldev(v.VanataObj["ldevId"].(string))
		r, err = MakeHTTPRequest("GET", url, nil, headers, "session", headers["Authorization"])
		if err != nil {
			fmt.Println("Failed to get LDEV info")
			return nil, err
		}
	case ADDPATH:
		var hostGroupId string
		url = api.Luns()
		body["ldevId"] = v.VanataObj["ldevId"].(string)
		for _, hostGroupId = range v.VanataObj["hostGroupIds"].([]string) {
			parts := strings.SplitN(hostGroupId, ",", 2)
			body["portId"] = parts[0]
			body["hostGroupNumber"] = parts[1]
			bodyJson, _ := json.Marshal(body)
			fmt.Println(string(bodyJson))
			_, err := api.InvokeAsyncCommand("POST", url, body, headers)
			if err != nil {
				fmt.Println("Failed to add path")
				return nil, err
			}
		}
	case DELETEPATH:
		var hostGroupId string
		var ldevEntry LdevEntry
		url = api.Ldev(v.VanataObj["ldevId"].(string))
		r, err = MakeHTTPRequest("GET", url, nil, headers, "session", headers["Authorization"])
		if err != nil {
			fmt.Println("Failed to get LDEV info")
			return nil, err
		}
		ldevEntryBytes, _ := json.Marshal(r)
		json.Unmarshal(ldevEntryBytes, &ldevEntry)
		fmt.Println(ldevEntry)
		for _, hostGroupId = range v.VanataObj["hostGroupIds"].([]string) {
			lunId, err := getlunID(ldevEntry, hostGroupId)
			if err != nil {
				fmt.Println("Failed to get LUN ID")
				return nil, err
			}
			objectID := hostGroupId + "," + lunId
			url = api.Lun(objectID)
			_, err = api.InvokeAsyncCommand("DELETE", url, body, headers)
			if err != nil {
				fmt.Println("Failed to delete path")
				return nil, err
			}
		}
	case GETPORTDETAILS:
		url = api.Ports() + "?detailInfoType=" + "logins"
		r, err = MakeHTTPRequest("GET", url, nil, headers, "session", headers["Authorization"])
		if err != nil {
			fmt.Println("Failed to get port details")
			return nil, err
		}
	default:
	}

	jsonData, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(jsonData))
	return r, nil

}
