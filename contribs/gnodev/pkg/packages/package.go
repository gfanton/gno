package packages

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/gnolang/gno/gnovm"
)

type PackageKind int

const (
	PackageKindOther  = iota
	PackageKindRemote = iota
	PackageKindFS
)

type Package struct {
	gnovm.MemPackage
	Kind     PackageKind
	Location string
}

func ReadPackageFromDir(fset *token.FileSet, path, dir string) (*Package, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to read dir %q: %w", dir, err)
	}

	var name string
	memFiles := []*gnovm.MemFile{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fname := file.Name()
		filepath := filepath.Join(dir, fname)
		body, err := os.ReadFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %q: %w", filepath, err)
		}

		if isGnoFile(fname) {
			memfile, pkgname, err := parseFile(fset, fname, body)
			if err != nil {
				return nil, fmt.Errorf("unable to parse file %q: %w", fname, err)
			}

			if !isTestFile(fname) {
				if name != "" && name != pkgname {
					return nil, fmt.Errorf("conflict package name between %q and %q", name, memfile.Name)
				}

				name = pkgname
			}

			memFiles = append(memFiles, memfile)
			continue // continue
		}

		if isValidPackageFile(fname) {
			memFiles = append(memFiles, &gnovm.MemFile{
				Name: fname, Body: string(body),
			})
		}

		// ignore the file
	}

	// Empty package, skipping
	if name == "" {
		return nil, nil
	}

	return &Package{
		MemPackage: gnovm.MemPackage{
			Name:  name,
			Path:  path,
			Files: memFiles,
		},
		Location: dir,
		Kind:     PackageKindFS,
	}, nil
}

func parseFile(fset *token.FileSet, fname string, body []byte) (*gnovm.MemFile, string, error) {
	f, err := parser.ParseFile(fset, fname, body, parser.PackageClauseOnly)
	if err != nil {
		return nil, "", fmt.Errorf("unable to parse file %q: %w", fname, err)
	}

	return &gnovm.MemFile{
		Name: fname,
		Body: string(body),
	}, f.Name.Name, nil
}
