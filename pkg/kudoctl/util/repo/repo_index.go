package repo

import (
	"fmt"
	"sort"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// IndexFile represents the index file in a framework repository
type IndexFile struct {
	APIVersion string                    `json:"apiVersion"`
	Generated  time.Time                 `json:"generated"`
	Entries    map[string]BundleVersions `json:"entries"`
}

// BundleVersions is a list of versioned bundle references.
// Implements a sorter on Version.
type BundleVersions []*BundleVersion

// BundleVersion represents a framework entry in the IndexFile
type BundleVersion struct {
	*Metadata
	URLs    []string  `json:"urls"`
	Created time.Time `json:"created,omitempty"`
	Removed bool      `json:"removed,omitempty"`
	Digest  string    `json:"digest,omitempty"`
}

// Len returns the length.
func (b BundleVersions) Len() int { return len(b) }

// Swap swaps the position of two items in the versions slice.
func (b BundleVersions) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// Less returns true if the version of entry a is less than the version of entry b.
func (b BundleVersions) Less(x, y int) bool {
	// Failed parse pushes to the back.
	i, err := semver.NewVersion(b[x].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(b[y].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}

// sortPackages sorts the entries by version in descending order.
//
// In canonical form, the individual version records should be sorted so that
// the most recent release for every version is in the 0th slot in the
// Entries.BundleVersions array. That way, tooling can predict the newest
// version without needing to parse SemVers.
func (i IndexFile) sortPackages() {
	for _, versions := range i.Entries {
		sort.Sort(sort.Reverse(versions))
	}
}

// parseIndexFile loads an index file and does minimal validity checking.
//
// This will fail if API Version is not set (ErrNoAPIVersion) or if the unmarshal fails.
func parseIndexFile(data []byte) (*IndexFile, error) {
	i := &IndexFile{}
	if err := yaml.Unmarshal(data, i); err != nil {
		return i, errors.Wrap(err, "unmarshalling index file")
	}
	i.sortPackages()
	if i.APIVersion == "" {
		return i, errors.New("no API version specified")
	}
	return i, nil
}

// GetByName returns the framework of given name.
func (i IndexFile) GetByName(name string) (*BundleVersion, error) {
	constraint, err := semver.NewConstraint("*")
	if err != nil {
		return nil, err
	}

	return i.getFramework(name, constraint)
}

// GetByNameAndVersion returns the framework of given name and version.
func (i IndexFile) GetByNameAndVersion(name, version string) (*BundleVersion, error) {
	constraint, err := semver.NewConstraint(version)
	if err != nil {
		return nil, err
	}

	return i.getFramework(name, constraint)
}

func (i IndexFile) getFramework(name string, versionConstraint *semver.Constraints) (*BundleVersion, error) {
	vs, ok := i.Entries[name]
	if !ok || len(vs) == 0 {
		return nil, fmt.Errorf("no framework of given name %s and version %v found", name, versionConstraint)
	}

	for _, ver := range vs {
		test, err := semver.NewVersion(ver.Version)
		if err != nil {
			continue
		}

		if versionConstraint.Check(test) {
			return ver, nil
		}
	}
	return nil, fmt.Errorf("no framework version found for %s-%v", name, versionConstraint)
}
