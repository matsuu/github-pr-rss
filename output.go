package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomically writes data to path, replacing any existing file
// atomically. The data is first written to a temporary file in the same
// directory, which is then renamed over path, so concurrent readers never
// observe a partially written file. Permissions of an existing file are
// preserved; new files are created with mode 0644.
func writeFileAtomically(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	mode := os.FileMode(0o644)
	if st, statErr := os.Stat(path); statErr == nil {
		if st.IsDir() {
			return fmt.Errorf("cannot write output to %s: it is a directory", path)
		}
		mode = st.Mode().Perm()
	}

	// The temporary file must live in the same directory so that the
	// rename below stays on one filesystem and is atomic.
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file in %s: %w", dir, err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("writing %s: %w", tmp.Name(), err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", tmp.Name(), err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmp.Name(), err)
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("setting permissions of %s to %v: %w", tmp.Name(), mode, err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
