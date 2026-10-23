// Package atomic writes files so that a crash or a full disk leaves either
// the old file or the new one, never half of each.
package atomic

import (
	"os"
	"path/filepath"
)

// WriteFile is os.WriteFile, but writes to a temporary file in the same
// directory and renames it into place. A symlink at path is followed, so a
// file kept in a dotfiles repository stays linked.
func WriteFile(path string, data []byte, perm os.FileMode) (err error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.new")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(f.Name())
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
