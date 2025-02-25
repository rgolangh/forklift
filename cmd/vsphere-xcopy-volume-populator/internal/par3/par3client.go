package par3

import (
	"bytes"
	"context"
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
}

type Par3ClientWsImpl struct {
	BaseURL    string
	SessionKey string
	Password   string
	Username   string
	HTTPClient *http.Client
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
	// Handle authentication errors explicitly
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		// Try to parse error response
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)

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

	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("Session expired (Code 6). Attempting to refresh session key...")
		if _, err := p.GetSessionKey(); err != nil {
			return fmt.Errorf("failed to refresh session key: %w", err)
		}

		req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)
		resp, err = p.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("retry request failed: %w", err)
		}
		defer resp.Body.Close()
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
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Members []struct {
			LUN      int    `json:"lun"`
			Hostname string `json:"hostname"`
		} `json:"members"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
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
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", p.SessionKey)

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Members []struct {
			VolumeName string `json:"volumeName"`
			LUN        int    `json:"lun"`
			Hostname   string `json:"hostname"`
		} `json:"members"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	for _, vlun := range response.Members {
		if vlun.VolumeName == lunName && vlun.Hostname == initiatorGroupName {
			return vlun.LUN, nil
		}
	}

	return 0, fmt.Errorf("LUN ID not found for volume %s and host %s", lunName, initiatorGroupName)
}
