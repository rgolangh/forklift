package testplan

import (
	"certificate-tool/internal/utils"
	"certificate-tool/pkg/storage"
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"k8s.io/client-go/kubernetes"
)

// TestPlan aggregates multiple test cases under a VM image.
type TestPlan struct {
	Image                string     `yaml:"image"`
	StorageVendorProduct string     `yaml:"storageVendorProduct"`
	TestCases            []TestCase `yaml:"tests"`
}

// Parse unmarshals YAML data into a TestPlan.
func Parse(yamlData []byte) (*TestPlan, error) {
	var tp TestPlan
	if err := yaml.Unmarshal(yamlData, &tp); err != nil {
		return nil, err
	}
	return &tp, nil
}

// Start runs all test cases sequentially, creating PVCs and pods, recording results.
func (tp *TestPlan) Start(ctx context.Context, clientset *kubernetes.Clientset, namespace, podImage, storageClassName, pvcYamlPath string) error {
	for i := range tp.TestCases {
		tc := &tp.TestCases[i]
		start := time.Now()
		if err := tc.Run(ctx, clientset, namespace, podImage, tp.Image, storageClassName, pvcYamlPath, tp.StorageVendorProduct); err != nil {
			tc.Results = utils.TestResult{false, int64(time.Since(start).Seconds()), err.Error()}
			return fmt.Errorf("test %s failed: %w", tc.Name, err)
		}
		tc.Results = utils.TestResult{true, int64(time.Since(start).Seconds()), ""}
	}
	return nil
}

// FormatOutput returns the marshaled YAML of metadata, image, and test results.
func (tp *TestPlan) FormatOutput() ([]byte, error) {
	output := struct {
		Metadata struct {
			Storage struct {
				storage.Storage
				StorageVendorProduct string `yaml:"storageVendorProduct"`
				ConnectionType       string `yaml:"connectionType"`
			} `yaml:"storage"`
		} `yaml:"metadata"`
		Image string     `yaml:"image"`
		Tests []TestCase `yaml:"tests"`
	}{
		Image: tp.Image,
		Tests: tp.TestCases,
	}

	c := storage.StorageCredentials{
		Hostname:      os.Getenv("STORAGE_HOSTNAME"),
		Username:      os.Getenv("STORAGE_USERNAME"),
		Password:      os.Getenv("STORAGE_PASSWORD"),
		SSLSkipVerify: os.Getenv("STORAGE_SKIP_SSL_VERIFICATION") == "true",
		VendorProduct: tp.StorageVendorProduct,
	}
	storageInfo, err  := storage.StorageInfo(c)
	if err != nil {
		return nil, err
	}
	output.Metadata.Storage.Storage = storageInfo
	output.Metadata.Storage.StorageVendorProduct = tp.StorageVendorProduct
	output.Metadata.Storage.ConnectionType = "TODOTODOOOOTODOTODDO"
	return yaml.Marshal(output)
}
