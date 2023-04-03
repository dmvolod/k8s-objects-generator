package apimachinery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/tools/go/ast/astutil"

	"github.com/kubewarden/k8s-objects-generator/download"
	"github.com/kubewarden/k8s-objects-generator/project"
)

const apimachineryRepo = "https://raw.githubusercontent.com/kubernetes/apimachinery/"

var apimachineryStaticFiles = []string{
	"/pkg/runtime/schema/group_version.go",
	"/pkg/runtime/schema/interfaces.go",
}

type staticContent struct {
	fs afero.Fs
}

func NewStaticContent(fs afero.Fs) *staticContent {
	return &staticContent{
		fs: fs,
	}
}

func (s *staticContent) CopyFiles(project project.Project) error {
	log.Println("============================================================================")
	log.Println("Generating static content files")
	release, err := project.ApimachineryRelease()
	if err != nil {
		return err
	}
	for _, staticLocation := range apimachineryStaticFiles {
		targetFilePath := filepath.Join(project.Root, "apimachinery", filepath.Join(strings.Split(staticLocation, "/")...))
		downloadUrl := apimachineryRepo + release + staticLocation
		fileData, err := download.FileContent(downloadUrl)
		if err != nil {
			return err
		}

		file, err := parser.ParseExprFrom(token.NewFileSet(), "", fileData, parser.ImportsOnly)
		if err != nil {
			return err
		}

		astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
			n := c.Node()
			switch x := n.(type) {
			case *ast.CallExpr:
				id, ok := x.Fun.(*ast.Ident)
				if ok {
					if id.Name == "pred" {
						c.Replace(&ast.UnaryExpr{
							Op: token.NOT,
							X:  x,
						})
					}
				}
			}

			return true
		})

		log.Println("File", downloadUrl, "downloaded into the", filepath.Dir(targetFilePath))
	}

	log.Println("============================================================================")

	return err
}
