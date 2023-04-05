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

	for _, location := range apimachineryStaticFiles {
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
		if err := s.modifySourceCode(fset, file, downloadUrl, targetPath(project.Root, location), project.GitRepo); err != nil {
			return err
		}
	}

	return nil
}

func (s *staticContent) dirStructMap(root string, locations []string) map[string]bool {
	uniql := make(map[string]bool)
	for _, loc := range locations {
		uniql[filepath.Dir(targetPath(root, loc))] = true
	}

	structs := make(map[string]bool)
	for loc := range uniql {
		files, _ := s.parseDir(token.NewFileSet(), loc, parser.ParseComments)
		for _, file := range files {
			for _, decl := range file.Decls {
				if structName := structDeclName(decl); len(structName) > 0 {
					structs[filepath.Join(loc, structName)] = true
				}
			}
		}
	}

	return structs
}

func structDeclName(decl ast.Decl) string {
	genDecl, ok := decl.(*ast.GenDecl)
	if !ok {
		return ""
	}
	for _, spec := range genDecl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if _, ok := typeSpec.Type.(*ast.StructType); ok {
			return typeSpec.Name.Name
		}
	}

	return ""
}

func targetPath(root, location string) string {
	return filepath.Join(root, "apimachinery", filepath.Join(strings.Split(location, "/")...))
}

func (s *staticContent) modifySourceCode(fset *token.FileSet, file *ast.File, downloadUrl, targetFilePath, getRepo string) error {
	titleComment := []*ast.CommentGroup{
		{
			List: []*ast.Comment{
				{
					Text: fmt.Sprintf("// Original file location %s\n", downloadUrl),
				},
			},
		},
	}

	for _, imp := range file.Imports {
		if strings.Contains(imp.Path.Value, "k8s.io") {
			imp.EndPos = imp.End()
			imp.Path.Value = strings.Replace(imp.Path.Value, "k8s.io", getRepo, 1)
		}
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			_, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
		}
	}

	file.Comments = append(titleComment, file.Comments...)
	if err := s.saveFile(fset, file, targetFilePath); err != nil {
		return err
	}
	log.Println("File", downloadUrl, "downloaded into the", filepath.Dir(targetFilePath))
	return nil
}

func (s *staticContent) parseDir(fset *token.FileSet, path string, mode parser.Mode) ([]*ast.File, error) {
	list, err := afero.ReadDir(s.fs, path)
	if err != nil {
		return nil, err
	}

	var files []*ast.File
	for _, d := range list {
		if d.IsDir() || !strings.HasSuffix(filepath.Base(d.Name()), ".go") {
			continue
		}
		if src, err := s.parseFile(fset, filepath.Join(path, d.Name()), mode); err == nil {
			files = append(files, src)
		} else {
			return nil, err
		}
	}

	return files, nil
}

func (s *staticContent) parseFile(fset *token.FileSet, filename string, mode parser.Mode) (f *ast.File, err error) {
	buf, err := afero.ReadFile(s.fs, filename)
	if err != nil {
		return nil, err
	}

	return parser.ParseFile(fset, filename, buf, mode)
}

func (s *staticContent) saveFile(fset *token.FileSet, file *ast.File, filePath string) error {
	if err := s.fs.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	targetFile, err := s.fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer targetFile.Close()
	return printer.Fprint(targetFile, fset, file)
}
