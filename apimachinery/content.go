package apimachinery

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/tools/go/ast/astutil"

	"github.com/kubewarden/k8s-objects-generator/download"
	"github.com/kubewarden/k8s-objects-generator/project"
)

const apimachineryRepo = "https://raw.githubusercontent.com/kubernetes/apimachinery/"

var apimachineryStaticFiles = []string{
	"/pkg/types/namespacedname.go",
	"/pkg/types/patch.go",
	"/pkg/types/uid.go",
	"/pkg/runtime/interfaces.go",
	"/pkg/runtime/types.go",
	"/pkg/runtime/schema/group_version.go",
	"/pkg/runtime/schema/interfaces.go",
	"/pkg/apis/meta/v1/types.go",
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
	defer log.Println("============================================================================")
	release, err := project.ApimachineryRelease()
	if err != nil {
		return err
	}
	if release == "" {
		log.Println("No Kubernetes release provided. Skipping static content files generating...")
		return nil
	}

	for _, staticLocation := range apimachineryStaticFiles {
		if err := s.downloadAndModify(staticLocation, release, project); err != nil {
			return err
		}
	}

	return nil
}

func (s *staticContent) downloadAndModify(location, release string, project project.Project) error {
	targetFilePath := filepath.Join(project.Root, "apimachinery", filepath.Join(strings.Split(location, "/")...))
	downloadUrl := apimachineryRepo + release + location
	fileData, err := download.FileContent(downloadUrl)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", fileData, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("unable to parse file, downloaded from %s: %w", downloadUrl, err)
	}

	dir, err := s.parseDir(token.NewFileSet(), filepath.Dir(targetFilePath), parser.ParseComments)
	println(dir)

	astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
		n := c.Node()
		switch x := n.(type) {
		case *ast.ImportSpec:
			if x.Path != nil && strings.Contains(x.Path.Value, "k8s.io") {
				x.Path.Value = strings.Replace(x.Path.Value, "k8s.io", project.GitRepo, 1)
				c.Replace(x)
			}
		}
		return true
	})

	titleComment := []*ast.CommentGroup{
		{
			List: []*ast.Comment{
				{
					Text: fmt.Sprintf("// Original file location %s\n", downloadUrl),
				},
			},
		},
	}

	file.Comments = append(titleComment, file.Comments...)
	if err := s.fs.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm); err != nil {
		return err
	}
	targetFile, err := s.fs.OpenFile(targetFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer targetFile.Close()
	if err := printer.Fprint(targetFile, fset, file); err != nil {
		return err
	}
	log.Println("File", downloadUrl, "downloaded into the", filepath.Dir(targetFilePath))
	return nil
}

func (s *staticContent) parseDir(fset *token.FileSet, path string, mode parser.Mode) (pkgs map[string]*ast.Package, first error) {
	dir, err := s.fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	list, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	pkgs = make(map[string]*ast.Package)
	for _, d := range list {
		if d.IsDir() || !strings.HasSuffix(filepath.Base(d.Name()), ".go") {
			continue
		}
		filename := filepath.Join(path, d.Name())
		if src, err := s.parseFile(fset, filename, mode); err == nil {
			name := src.Name.Name
			pkg, found := pkgs[name]
			if !found {
				pkg = &ast.Package{
					Name:  name,
					Files: make(map[string]*ast.File),
				}
				pkgs[name] = pkg
			}
			pkg.Files[filename] = src
		} else if first == nil {
			first = err
		}
	}

	return
}

func (s *staticContent) parseFile(fset *token.FileSet, filename string, mode parser.Mode) (f *ast.File, err error) {
	file, err := s.fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var buf []byte
	if _, err := file.Read(buf); err != nil {
		return nil, err
	}

	return parser.ParseFile(fset, "", buf, mode)
}
