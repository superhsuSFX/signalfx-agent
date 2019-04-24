package monitors

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const monitorMetadataFile = "metadata.yaml"

// MetricMetadata contains a metric's metadata.
type MetricMetadata struct {
	Name        string  `json:"name"`
	Alias       string  `json:"alias,omitempty"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Group       *string `json:"group"`
	Included    bool    `json:"included" default:"false"`
}

// PropMetadata contains a property's metadata.
type PropMetadata struct {
	Name        string `json:"name"`
	Dimension   string `json:"dimension"`
	Description string `json:"description"`
}

// GroupMetadata contains a group's metadata.
type GroupMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type metricsYaml struct {
	Metrics []MetricMetadata
}

func (my *metricsYaml) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var metricsMap map[string][]MetricMetadata

	if err := unmarshal(&my.Metrics); err == nil {
		return nil
	}

	if err := unmarshal(&metricsMap); err == nil {
		for _, metrics := range metricsMap {
			my.Metrics = append(my.Metrics, metrics...)
		}
		return nil
	}

	return errors.New("unable deserialize metrics key")
}

// MonitorMetadata contains a monitor's metadata.
type MonitorMetadata struct {
	MonitorType string           `json:"monitorType" yaml:"monitorType"`
	SendAll     bool             `json:"sendAll" yaml:"sendAll"`
	Dimensions  []DimMetadata    `json:"dimensions"`
	Doc         string           `json:"doc"`
	Groups      []GroupMetadata  `json:"groups"`
	Metrics     []MetricMetadata `json:"-" yaml:"-"`
	MetricsYaml metricsYaml      `json:"metrics" yaml:"metrics"`
	Properties  []PropMetadata   `json:"properties"`
	// True if the list of metrics is definitively the set of metrics
	// this monitor will ever send. This impacts the additionalMetricsFilter.
	MetricsExhaustive bool `json:"metricsExhaustive" yaml:"metricsExhaustive" default:"false"`
}

// PackageMetadata describes a package directory that may have one or more monitors.
type PackageMetadata struct {
	PackageDir string `json:"packageDir" yaml:"packageDir"`
	Monitors   []MonitorMetadata
	// Name of the package in go. If not set defaults to the directory name.
	GoPackage *string `json:"goPackage" yaml:"goPackage"`
	// Filesystem path to the package directory.
	PackagePath string `json:"-" yaml:"-"`
	Path        string `json:"-" yaml:"-"`
}

// DimMetadata contains a dimension's metadata.
type DimMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CollectMetadata loads metadata for all monitors located in root as well as any subdirectories.
func CollectMetadata(root string) ([]PackageMetadata, error) {
	var packages []PackageMetadata

	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || info.Name() != monitorMetadataFile {
			return nil
		}

		var pkg PackageMetadata

		if bytes, err := ioutil.ReadFile(path); err != nil {
			return errors.Wrapf(err, "unable to read metadata file %s", path)
		} else if err := yaml.UnmarshalStrict(bytes, &pkg); err != nil {
			return errors.Wrapf(err, "unable to unmarshal file %s", path)
		}

		for i, monitor := range pkg.Monitors {
			monitor.Metrics = monitor.MetricsYaml.Metrics
			pkg.Monitors[i] = monitor
		}

		pkg.PackagePath = filepath.Dir(path)
		pkg.Path = path

		packages = append(packages, pkg)

		return nil
	}); err != nil {
		return nil, err
	}

	return packages, nil
}
