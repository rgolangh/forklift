package par3

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"io"
	"log"
	"net/http"
)

type Par3Client interface {
	GetSessionKey() (string, error)
	EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) error
	LunUnmap(ctx context.Context, initiatorGroupName, lunName string) error
	EnsureHostWithIqn(initiatorGroupName string, iqn string) error
	EnsureHostSetExists(hostSetName string) error
	AddHostToHostSet(hostSetName string, hostName string) error
	GetLunDetailsByVolumeName(lunName string) (string, string, error)
	CurrentMappedGroups(volumeName string) ([]string, error)
}

type Par3ClientWsImpl struct {
	BaseURL    string
	SessionKey string
	Password   string
	Username   string
	HTTPClient *http.Client
}

func NewPar3ClientWsImpl(storageHostname, storageUsername, storagePassword string) Par3ClientWsImpl {
	return Par3ClientWsImpl{
		BaseURL:  storageHostname,
		Password: storagePassword,
		Username: storageUsername,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Disable SSL verification
			},
		},
	}
}

func (p *Par3ClientWsImpl) EnsureHostWithIqn(initiatorGroupName string, iqn string) error {
	exists, err := p.hostExists(initiatorGroupName)
	if err != nil {
		return fmt.Errorf("failed to check host existence: %w", err)
	}

	if exists {
		return nil
	}

	return p.createHost(initiatorGroupName, iqn)
}

func (p *Par3ClientWsImpl) hostExists(hostname string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/hosts/%s", p.BaseURL, hostname)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode, string(body))
}

func (p *Par3ClientWsImpl) createHost(hostname, iqn string) error {
	url := fmt.Sprintf("%s/api/v1/hosts", p.BaseURL)

	requestBody := map[string]interface{}{
		"name":    hostname,
		"persona": 2,
		"iSCSIPaths": []map[string]string{
			{"iqn": iqn},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.doRequest(req, "createHost")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to create host: status %d, body: %s", resp.StatusCode, string(body))
}

func (p *Par3ClientWsImpl) GetSessionKey() (string, error) {
	url := fmt.Sprintf("%s/api/v1/credentials", p.BaseURL)

	requestBody := map[string]string{
		"user":     p.Username,
		"password": p.Password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		var errorResp struct {
			Code int    `json:"code"`
			Desc string `json:"desc"`
		}

		if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
			return "", fmt.Errorf("authentication failed: %s (code %d)", errorResp.Desc, errorResp.Code)
		}
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response map[string]string
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse session key response: %w", err)
	}

	if sessionKey, ok := response["key"]; ok {
		p.SessionKey = sessionKey
		log.Println("Successfully obtained new session key")
		return sessionKey, nil
	}

	return "", fmt.Errorf("failed to retrieve session key, response: %s", string(bodyBytes))
}

func (p *Par3ClientWsImpl) EnsureLunMapped(initiatorGroup string, targetLUN populator.LUN) error {
	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)

	lunID, err := p.GetFreeLunID(initiatorGroup)
	if err != nil {
		return err
	}

	requestBody := map[string]interface{}{
		"volumeName": targetLUN.Name,
		"lun":        lunID,
		"hostname":   fmt.Sprintf("set:%s", initiatorGroup),
		"autoLun":    true,
		"maxAutoLun": 200,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Println(resp)
		return fmt.Errorf("failed to map LUN: status %d", resp.StatusCode)
	}

	return nil
}

func (p *Par3ClientWsImpl) LunUnmap(ctx context.Context, initiatorGroupName, lunName string) error {
	lunID, err := p.GetLunID(lunName, fmt.Sprintf("set:%s", initiatorGroupName))
	if err != nil {
		return fmt.Errorf("failed to get LUN ID: %w", err)
	}

	fields := map[string]interface{}{
		"LUN":         lunName,
		"igroup":      initiatorGroupName,
		"LUN ID Used": lunID,
	}

	log.Printf("LunUnmap: %v", fields)

	url := fmt.Sprintf("%s/api/v1/vluns/%s,%d,%s", p.BaseURL, lunName, lunID, fmt.Sprintf("set:%s", initiatorGroupName))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	resp, err = p.handleUnauthorizedSessionKey(resp, req, err)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to unmap LUN: status %d", resp.StatusCode)
	}

	log.Printf("LunUnmap: Successfully unmapped LUN %s from %s", lunName, initiatorGroupName)
	return nil
}

func (p *Par3ClientWsImpl) GetFreeLunID(initiatorGroupName string) (int, error) {
	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	var response struct {
		Members []struct {
			LUN      int    `json:"lun"`
			Hostname string `json:"hostname"`
		} `json:"members"`
	}
	err = p.doRequestUnmarshalResponse(req, "getFreeLunId", &response)
	if err != nil {
		return 0, err
	}

	usedLUNs := make(map[int]bool)
	for _, vlun := range response.Members {
		if vlun.Hostname == initiatorGroupName {
			usedLUNs[vlun.LUN] = true
		}
	}

	for i := 1; i <= 255; i++ {
		if !usedLUNs[i] {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no available LUN ID found for host %s", initiatorGroupName)
}

func (p *Par3ClientWsImpl) GetLunID(lunName, initiatorGroupName string) (int, error) {
	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	var response struct {
		Members []struct {
			VolumeName string `json:"volumeName"`
			LUN        int    `json:"lun"`
			Hostname   string `json:"hostname"`
		} `json:"members"`
	}

	err = p.doRequestUnmarshalResponse(req, "getLunId", &response)
	if err != nil {
		return 0, err
	}
	for _, vlun := range response.Members {
		if vlun.VolumeName == lunName && vlun.Hostname == initiatorGroupName {
			return vlun.LUN, nil
		}
	}

	return 0, fmt.Errorf("LUN ID not found for volume %s and host %s", lunName, initiatorGroupName)
}

func (p *Par3ClientWsImpl) GetLunDetailsByVolumeName(volumeName string) (name string, serialNumber string, err error) {
	cutVolName := prefixOfString(volumeName, 31)
	url := fmt.Sprintf("%s/api/v1/volumes/%s", p.BaseURL, cutVolName)

	reqType := "getVolume"
	fmt.Println(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	type MyResponse struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
		WWN  string `json:"wwn"`
	}

	var response MyResponse

	err = p.doRequestUnmarshalResponse(req, reqType, &response)
	if err != nil {
		return "", "", err
	}
	fmt.Println(response)
	fmt.Println(">>>>>>>>>>>>>>>>>>>")

	if response.Name != "" {
		return cutVolName, serialNumber, nil
	}
	return "", "", fmt.Errorf("volume not found for volume: %s", cutVolName)
}

func (p *Par3ClientWsImpl) CurrentMappedGroups(volumeName string) ([]string, error) {
	type VLUN struct {
		LUN        int    `json:"lun"`
		VolumeName string `json:"volumeName"`
		Hostname   string `json:"hostname"`
	}

	type Response struct {
		Members []VLUN `json:"members"`
	}

	var response Response

	url := fmt.Sprintf("%s/api/v1/vluns", p.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to create request: %w", err)
	}
	err = p.doRequestUnmarshalResponse(req, "GET", &response)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VLUNs: %w", err)
	}

	hostnameSet := make(map[string]struct{})

	for _, vlun := range response.Members {
		if vlun.VolumeName == volumeName {
			hostnameSet[vlun.Hostname] = struct{}{}
		}
	}

	hostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		hostnames = append(hostnames, hostname)
	}

	return hostnames, nil
}

func (p *Par3ClientWsImpl) doRequest(req *http.Request, reqDescription string) (*http.Response, error) {
	_, err := p.GetSessionKey()
	if err != nil {
		return nil, err
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", reqDescription, err)
	}

	if resp, err = p.handleUnauthorizedSessionKey(resp, req, err); err != nil {
		return nil, fmt.Errorf("failed for %s: %w", reqDescription, err)
	}

	return resp, nil
}

func (p *Par3ClientWsImpl) doRequestUnmarshalResponse(req *http.Request, reqDescription string, response interface{}) error {
	_, err := p.GetSessionKey()
	if err != nil {
		return err
	}

	p.setReqHeadersWithSessionKey(req)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed for %s: %w", reqDescription, err)
	}
	defer resp.Body.Close()

	if resp, err = p.handleUnauthorizedSessionKey(resp, req, err); err != nil {
		return fmt.Errorf("failed for %s: %w", reqDescription, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed for %s: status %d, body: %s", reqDescription, resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response for %s: %w", reqDescription, err)
	}

	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("failed to parse JSON for %s: %w", reqDescription, err)
	}

	return nil
}

func (p *Par3ClientWsImpl) handleUnauthorizedSessionKey(resp *http.Response, req *http.Request, err error) (*http.Response, error) {
	if resp.StatusCode == http.StatusUnauthorized {
		if _, err := p.GetSessionKey(); err != nil {
			return nil, fmt.Errorf("failed to refresh session key: %w", err)
		}

		p.setReqHeadersWithSessionKey(req)
		resp, err = p.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
		defer resp.Body.Close()
	}
	return resp, nil
}

func (p *Par3ClientWsImpl) EnsureHostSetExists(hostSetName string) error {
	url := fmt.Sprintf("%s/api/v1/hostsets/%s", p.BaseURL, hostSetName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := p.doRequest(req, "ensureHostSetExists, find set")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil // Host set already exists
	}

	createURL := fmt.Sprintf("%s/api/v1/hostsets", p.BaseURL)
	requestBody := map[string]interface{}{
		"name": hostSetName,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err = http.NewRequest("POST", createURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	respCreate, err := p.doRequest(req, "EnsuresHostSetExists")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(respCreate.Body)
		return fmt.Errorf("failed to create host set: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *Par3ClientWsImpl) setReqHeadersWithSessionKey(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)
}

func (p *Par3ClientWsImpl) AddHostToHostSet(hostSetName string, hostName string) error {
	url := fmt.Sprintf("%s/api/v1/hostsets/%s", p.BaseURL, hostSetName)

	requestBody := map[string]interface{}{
		"action": "add",
		"setmembers": []string{
			hostName,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.doRequest(req, "AddHostToHostSet")

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add host to host set: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func prefixOfString(s string, length int) string {
	runes := []rune(s)
	if len(runes) > length {
		return string(runes[:length])
	}
	return s
}
