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
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/tools/go/ast/astutil"

	"github.com/kubewarden/k8s-objects-generator/download"
	"github.com/kubewarden/k8s-objects-generator/project"
)

const apimachineryRepo = "https://raw.githubusercontent.com/kubernetes/apimachinery/"

var apimachineryStaticFiles = []string{
	"/pkg/apis/meta/v1/types.go",
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

	pkgs, _ := s.parseDir(token.NewFileSet(), filepath.Dir(targetFilePath), parser.ParseComments)

	astutil.Apply(file, func(c *astutil.Cursor) bool {
		n := c.Node()
		switch x := n.(type) {
		case *ast.TypeSpec:
			if isDuplicateStruct(x, pkgs) {
				c.Delete()
			}
		}
		return true
	}, nil)

	titleComment := []*ast.CommentGroup{
		{
			List: []*ast.Comment{
				{
					Text: fmt.Sprintf("// Original file location %s\n", downloadUrl),
				},
			},
		},
	}

	for _, group := range astutil.Imports(fset, file) {
		for _, spec := range group {
			if strings.Contains(spec.Path.Value, "k8s.io") {
				oldPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				astutil.RewriteImport(fset, file, oldPath, strings.Replace(oldPath, "k8s.io", project.GitRepo, 1))
			}
		}
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
	list, err := afero.ReadDir(s.fs, path)
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
	buf, err := afero.ReadFile(s.fs, filename)
	if err != nil {
		return nil, err
	}

	return parser.ParseFile(fset, filename, buf, mode)
}

func isDuplicateStruct(entry *ast.TypeSpec, pkgs map[string]*ast.Package) (ret bool) {
	if pkgs == nil {
		return false
	}

	for _, pkg := range pkgs {
		astutil.Apply(pkg, nil, func(c *astutil.Cursor) bool {
			n := c.Node()
			switch x := n.(type) {
			case *ast.TypeSpec:
				if x.Name.Name == entry.Name.Name {
					ret = true
					return false
				}
			}
			return true
		})
	}

	return
}
