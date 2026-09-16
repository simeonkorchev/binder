// Package sentinelscan reads a package's exported error sentinels out of its
// source.
//
// It exists for one test per domain: the one that proves every sentinel a
// service can return has a decided HTTP status in that domain's api/errors.go
// table (000-principles.md section 8c). That test must enumerate the sentinels
// from the source, because a hand-written list stops being true the day someone
// adds one — and the failure mode it guards against is precisely a sentinel
// nobody remembered (006-testing.md, "Derive lists from source, not by hand").
//
// Go has no reflection over a package's declarations, so this parses them.
package sentinelscan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// errSentinelShape reports a sentinel this package cannot read a message from.
// It is a broken assumption in the scanned package rather than a test failure to
// be worked around: 002-go-conventions.md section 4.1 requires sentinels to be
// declared with errors.New.
var errSentinelShape = errors.New("exported sentinel is not declared with errors.New")

// sentinelPrefix is what 002-go-conventions.md section 4.1 requires an error
// variable's name to start with, and errname enforces.
const sentinelPrefix = "Err"

// ExportedMessages returns the message of every exported Err... sentinel
// declared at package scope in dir, keyed by the variable's name. Test files
// are skipped: a sentinel a test invents is not one a service can return.
func ExportedMessages(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	fileSet := token.NewFileSet()
	messages := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !isSourceFile(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if err := collectFileSentinels(file, messages); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

// isSourceFile skips test files: a sentinel a test invents is not one a service
// can return.
func isSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func collectFileSentinels(file *ast.File, into map[string]string) error {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			if err := collectSpecSentinels(spec, into); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectSpecSentinels(spec ast.Spec, into map[string]string) error {
	valueSpec, ok := spec.(*ast.ValueSpec)
	if !ok {
		return nil
	}

	for i, name := range valueSpec.Names {
		if !isExportedSentinel(name.Name) || i >= len(valueSpec.Values) {
			continue
		}
		message, ok := errorsNewMessage(valueSpec.Values[i])
		if !ok {
			return fmt.Errorf("%w: %s", errSentinelShape, name.Name)
		}
		into[name.Name] = message
	}
	return nil
}

func isExportedSentinel(name string) bool {
	return strings.HasPrefix(name, sentinelPrefix) && name != sentinelPrefix
}

// errorsNewMessage reads the literal message out of an `errors.New("...")` call.
func errorsNewMessage(value ast.Expr) (string, bool) {
	call, ok := value.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "New" {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "errors" {
		return "", false
	}

	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	message, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return message, true
}
