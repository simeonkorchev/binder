package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/testdb"
)

// binaryPath and databaseURL are the suite's shared fixtures: the compiled
// server, and an isolated migrated database for it to open. Both are taken
// before RunSpecs because building and provisioning need the *testing.T — to
// register cleanup, and to skip the suite when no Postgres is reachable.
//
//nolint:gochecknoglobals // suite-shared fixtures, as in every store suite (002-go-conventions.md section 9).
var (
	binaryPath  string
	databaseURL string
)

func TestBinderd(t *testing.T) {
	databaseURL = testdb.NewURL(t)
	binaryPath = buildServer(t)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Binderd Suite")
}

// buildServer compiles the command under test and returns the binary's path.
//
// The specs run the real executable rather than calling into this package,
// because what they are here to prove is the part no unit test reaches: that a
// process with this configuration starts, binds a socket, and answers requests
// that arrived over it.
func buildServer(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "binderd")
	// The package is the one this test file is in, so there is no path from a
	// request or a row anywhere in this command.
	build := exec.CommandContext(context.Background(), "go", "build", "-o", path, ".")
	build.Env = os.Environ()

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the server: %v\n%s", err, out)
	}
	return path
}
