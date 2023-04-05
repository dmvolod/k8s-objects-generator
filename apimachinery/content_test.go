package apimachinery

import (
	"reflect"
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
	content := NewStaticContent(fs)
	assert.NoError(t, content.CopyFiles(project))
}

func TestStructExtraction(t *testing.T) {
	testParse := []string{
		"testdata/parse/test.go",
	}
	expectedMap := map[string]bool{
		"../apimachinery/testdata/parse/first":  true,
		"../apimachinery/testdata/parse/second": true,
	}

	content := NewStaticContent(afero.NewOsFs())
	assert.True(t, reflect.DeepEqual(expectedMap, content.dirStructMap("..", testParse)), "The generated structure maps must be equal")
}
