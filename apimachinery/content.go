package apimachinery

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/kubewarden/k8s-objects-generator/download"
	"github.com/kubewarden/k8s-objects-generator/project"
)

const apimachineryRepo = "https://raw.githubusercontent.com/kubernetes/apimachinery/"

var StaticFiles = []string{
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
	fs          afero.Fs
	extractor   *sourceExtractor
	staticFiles []string
	project     project.Project
}

type sourceExtractor struct {
	structs map[string]map[string]bool
}

func NewSourceExtractor(fs afero.Fs, root string, locations []string) *sourceExtractor {
	uniql := make(map[string]bool)
	for _, loc := range locations {
		uniql[filepath.Dir(targetPath(root, loc))] = true
	}

	structs := make(map[string]map[string]bool)
	for loc := range uniql {
		files, _ := parseDir(fs, token.NewFileSet(), loc, parser.ParseComments)
		structs[loc] = make(map[string]bool)
		for _, file := range files {
			for _, decl := range file.Decls {
				if structName := structDeclName(decl); len(structName) > 0 {
					structs[loc][structName] = true
				}
			}
		}
	}

	return &sourceExtractor{
		structs,
	}
}

func (se sourceExtractor) IsStructExist(location, name string) bool {
	return se.structs[location][name]
}

func NewStaticContent(fs afero.Fs, project project.Project, staticFiles []string) *staticContent {
	return &staticContent{
		fs:          fs,
		staticFiles: staticFiles,
		extractor:   NewSourceExtractor(fs, project.Root, staticFiles),
		project:     project,
	}
}

func (s *staticContent) CopyFiles() error {
	log.Println("============================================================================")
	log.Println("Generating static content files")
	defer log.Println("============================================================================")
	release, err := s.project.ApimachineryRelease()
	if err != nil {
		return err
	}
	if release == "" {
		log.Println("No Kubernetes release provided. Skipping static content files generating...")
		return nil
	}

	for _, location := range s.staticFiles {
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
		targetFilePath := targetPath(s.project.Root, location)
		if err = s.modifySourceCode(file, downloadUrl, targetFilePath); err != nil {
			return err
		}
		if err = s.saveFile(fset, file, targetFilePath); err != nil {
			return err
		}
		log.Println("File", downloadUrl, "downloaded into the", filepath.Dir(targetFilePath))
	}

	return nil
}

func (s *staticContent) modifySourceCode(file *ast.File, downloadUrl, targetFilePath string) error {
	titleComment := []*ast.CommentGroup{
		{
			List: []*ast.Comment{
				{
					Slash: token.Pos(1),
					Text:  fmt.Sprintf("// Original file location %s\n", downloadUrl),
				},
			},
		},
	}

	for _, imp := range file.Imports {
		if strings.Contains(imp.Path.Value, "k8s.io") {
			imp.EndPos = imp.End()
			imp.Path.Value = strings.Replace(imp.Path.Value, "k8s.io", s.project.GitRepo, 1)
		}
	}

	astutil.Apply(file, func(c *astutil.Cursor) bool {
		n := c.Node()
		if d, ok := n.(*ast.GenDecl); ok && len(d.Specs) > 0 {
			if t, ok := d.Specs[0].(*ast.TypeSpec); ok && s.extractor.IsStructExist(filepath.Dir(targetFilePath), t.Name.Name) {
				c.Delete()
				deleteComments(file, d.Pos(), d.End())
			}
		}
		return true
	}, nil)

	file.Comments = append(titleComment, file.Comments...)
	return nil
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

func targetPath(root, location string) string {
	return filepath.Join(root, "apimachinery", filepath.Join(strings.Split(location, "/")...))
}

func deleteComments(file *ast.File, pos, end token.Pos) {
	var newComments []*ast.CommentGroup
	for _, com := range file.Comments {
		if !((com.Pos() >= pos && com.End() <= end) || (com.End()+1 == pos)) {
			newComments = append(newComments, com)
		}
	}
	file.Comments = newComments
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

func parseDir(fs afero.Fs, fset *token.FileSet, path string, mode parser.Mode) ([]*ast.File, error) {
	list, err := afero.ReadDir(fs, path)
	if err != nil {
		return nil, err
	}

	var files []*ast.File
	for _, d := range list {
		if d.IsDir() || !strings.HasSuffix(filepath.Base(d.Name()), ".go") {
			continue
		}
		if src, err := parseFile(fs, fset, filepath.Join(path, d.Name()), mode); err == nil {
			files = append(files, src)
		} else {
			return nil, err
		}
	}

	return files, nil
}

func parseFile(fs afero.Fs, fset *token.FileSet, filename string, mode parser.Mode) (f *ast.File, err error) {
	buf, err := afero.ReadFile(fs, filename)
	if err != nil {
		return nil, err
	}

	return parser.ParseFile(fset, filename, buf, mode)
}
