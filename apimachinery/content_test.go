package apimachinery

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/kubewarden/k8s-objects-generator/project"
)

func TestModifySourceCode(t *testing.T) {
	outputDir := "/testout"
	project, err := project.NewProject(outputDir, "github.com/kubewarden/k8s-objects", "", "1.24")
	assert.NoError(t, err)

	fs := afero.NewMemMapFs()
	content := NewStaticContent(fs, project)
	assert.NoError(t, content.CopyFiles())
}

func TestSourceExtractor(t *testing.T) {
	const (
		expectedLocation   = "../apimachinery/testdata/parse"
		unexpectedLocation = "not/defined/location"
	)

	testParse := []string{
		"testdata/parse/test.go",
	}

	sources := NewSourceExtractor(afero.NewOsFs(), "..", testParse)
	assert.True(t, sources.IsStructExist(expectedLocation, "first"))
	assert.True(t, sources.IsStructExist(expectedLocation, "second"))
	assert.False(t, sources.IsStructExist(expectedLocation, "iface"))
	assert.False(t, sources.IsStructExist(unexpectedLocation, "first"))
}
