package apimachinery

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/kubewarden/k8s-objects-generator/project"
)

func TestCopyStaticContent(t *testing.T) {
	outputDir := "/testout"
	project, err := project.NewProject(outputDir, "github.com/kubewarden/k8s-objects", "", "1.24")
	assert.NoError(t, err)

	fs := afero.NewMemMapFs()
	content := NewStaticContent(fs)
	assert.NoError(t, content.CopyFiles(project))
}
