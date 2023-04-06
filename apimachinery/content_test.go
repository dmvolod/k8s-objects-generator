package apimachinery

import (
	_ "embed"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/kubewarden/k8s-objects-generator/project"
)

//go:embed testdata/parse/test-copy.go.orig
var testCopyFile string

//go:embed testdata/parse/test.go
var testFile string

func TestModifySourceCode(t *testing.T) {
	project, err := project.NewProject("/testout", "github.com/kubewarden/k8s-objects", "", "")
	assert.NoError(t, err)

	fs := afero.NewMemMapFs()
	testParse := []string{
		"testdata/parse/test.go",
	}

	assert.NoError(t, afero.WriteFile(fs, targetPath(project.Root, "testdata/parse/test.go"), []byte(testFile), os.ModePerm))
	content := NewStaticContent(fs, project, testParse)
	file, err := parser.ParseFile(token.NewFileSet(), "", testCopyFile, parser.ParseComments)
	assert.NoError(t, err)
	//printer.Fprint(os.Stdout, token.NewFileSet(), file)

	assert.NoError(t, content.modifySourceCode(token.NewFileSet(), file, "http://mock.url", targetPath(project.Root, "testdata/parse/test-copy.go")))
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
