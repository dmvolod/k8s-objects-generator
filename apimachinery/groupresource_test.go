package apimachinery

import (
	_ "embed"

	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/kubewarden/k8s-objects-generator/project"
	"github.com/kubewarden/k8s-objects-generator/split"
)

//go:embed testdata/event_gvk.go.gold
var eventGvkGold string

//go:embed testdata/group_info.go.gold
var groupInfoGold string

func TestKubernetesExtensionParse(t *testing.T) {
	tests := []struct {
		extensionJson   string
		extensionParsed bool
		expectedGVK     *groupVersionResource
	}{
		{
			extensionJson: `{"x-kubernetes-group-version-kind": [
        						{
          							"group": "events.k8s.io",
									"kind": "Event",
          							"version": "v1"
        						}
							]}`,
			extensionParsed: true,
			expectedGVK: &groupVersionResource{
				Group:   "events.k8s.io",
				Version: "v1",
				Kind:    "Event",
			},
		},
		{
			extensionJson: `{"x-kubernetes-group-version-kind": [
        						{
          							"group": "",
          							"kind": "DeleteOptions",
          							"version": "v1"
        						},
        						{
          							"group": "admission.k8s.io",
          							"kind": "DeleteOptions",
          							"version": "v1"
        						}
							]}`,
			extensionParsed: false,
		},
	}

	for _, tt := range tests {
		extension := spec.VendorExtensible{}
		assert.NoError(t, extension.UnmarshalJSON([]byte(tt.extensionJson)))
		kubeExtension, isKubeExtension := asKubernetesExtension(extension.Extensions)
		assert.Equal(t, isKubeExtension, tt.extensionParsed)
		if tt.extensionParsed {
			assert.Equal(t, kubeExtension[kubernetesGroupKey], tt.expectedGVK.Group)
			assert.Equal(t, kubeExtension[kubernetesVersionKey], tt.expectedGVK.Version)
			assert.Equal(t, kubeExtension[kubernetesKindKey], tt.expectedGVK.Kind)
		}
	}
}

func TestGenerateGroupResources(t *testing.T) {
	outputDir := "/testout"
	project, err := project.NewProject(outputDir, "", "", "1.24")
	assert.NoError(t, err)

	splitter, err := split.NewSplitter(filepath.Join("testdata", "test-swagger.json"))
	assert.NoError(t, err)

	refactoringPlan, err := splitter.ComputeRefactoringPlan()
	assert.NoError(t, err)

	fs := afero.NewMemMapFs()
	groupResource := NewGroupResource(fs)
	assert.NoError(t, groupResource.Generate(project, refactoringPlan))

	eventGvk, err := afero.ReadFile(fs, filepath.Join(outputDir, "src/api/events/v1/event_gvk.go"))
	assert.NoError(t, err)
	groupInfo, err := afero.ReadFile(fs, filepath.Join(outputDir, "src/api/events/v1/group_info.go"))
	assert.NoError(t, err)

	assert.Equal(t, eventGvkGold, string(eventGvk))
	assert.Equal(t, groupInfoGold, string(groupInfo))
}
