package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
)

func filepathFSOpen(root, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	return os.Open(filepath.Join(root, filepath.FromSlash(name)))
}
