package apimachinery

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

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
		data, err := download.FileDownload(downloadUrl)
		if err != nil {
			return err
		}
		println(data)
		log.Println("File", downloadUrl, "downloaded into the", filepath.Dir(targetFilePath))
	}

	log.Println("============================================================================")

	return err
}
