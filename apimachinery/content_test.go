package apimachinery

import (
	"bytes"
	_ "embed"
	"go/parser"
	"go/printer"
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

//go:embed testdata/parse/test-copy.go.gold
var testCopyGoldFile string

func TestModifySourceCode(t *testing.T) {
	project, err := project.NewProject("/testout", "github.com/kubewarden/k8s-objects", "", "")
	assert.NoError(t, err)

	fs := afero.NewMemMapFs()
	testParse := []string{
		"testdata/parse/test.go",
	}

	assert.NoError(t, afero.WriteFile(fs, targetPath(project.Root, "testdata/parse/test.go"), []byte(testFile), os.ModePerm))
	content := NewStaticContent(fs, project, testParse)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", testCopyFile, parser.ParseComments)
	assert.NoError(t, err)

	var buf bytes.Buffer
	assert.NoError(t, content.modifySourceCode(file, "http://mock.url", targetPath(project.Root, "testdata/parse/test-copy.go")))
	assert.NoError(t, printer.Fprint(&buf, fset, file))
	assert.Equal(t, testCopyGoldFile, buf.String())
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
