package objectstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// Dir stores objects as files under a root directory, mirroring the key's
// slashes as subdirectories. It is what makes the image pipeline runnable
// without cloud credentials — in the suites, and on a machine that has egress
// to the card API but no service account.
type Dir struct {
	root string
}

// NewDir creates the root if it does not exist, so a caller does not have to
// prepare the directory before the first Put.
func NewDir(root string) (*Dir, error) {
	if root == "" {
		return nil, fmt.Errorf("creating directory object store: %w", ErrInvalidKey)
	}
	if err := os.MkdirAll(root, dirPerm); err != nil {
		return nil, fmt.Errorf("creating object store root: %w", err)
	}
	return &Dir{root: root}, nil
}

// Put writes body at root/key, creating the key's parent directories.
//
// The fourth parameter is the content type. Dir takes it so it satisfies the
// same contract as GCS and ignores it on purpose: a filesystem has nowhere to
// record it, and nothing serves objects from here — the extension in the key
// is what a later upload to the real bucket derives it from.
func (d *Dir) Put(ctx context.Context, key string, body []byte, _ string) error {
	if err := validateKey(key); err != nil {
		return fmt.Errorf("putting object %q: %w", key, err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("putting object %q: %w", key, err)
	}

	target := filepath.Join(d.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return fmt.Errorf("creating object directory for %q: %w", key, err)
	}
	if err := os.WriteFile(target, body, filePerm); err != nil {
		return fmt.Errorf("writing object %q: %w", key, err)
	}
	return nil
}
