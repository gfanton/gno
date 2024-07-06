package dev

import (
	"fmt"
	"path/filepath"

	"github.com/gnolang/gno/gnovm/pkg/gnomod"
)

type pkgsLoader struct {
	pkgs    []gnomod.Pkg
	visited map[string]struct{}
}

func newPkgsLoader() *pkgsLoader {
	return &pkgsLoader{
		pkgs:    make([]gnomod.Pkg, 0),
		visited: make(map[string]struct{}),
	}
}

func (pl *pkgsLoader) List() gnomod.PkgList {
	return pl.pkgs
}

func (pl *pkgsLoader) LoadAllPackagesFromDir(path string) error {
	// list all packages from target path
	pkgslist, err := gnomod.ListPkgs(path)
	if err != nil {
		return fmt.Errorf("listing gno packages: %w", err)
	}

	for _, pkg := range pkgslist {
		if !pl.exist(pkg) {
			pl.add(pkg)
		}
	}

	return nil
}

func (pl *pkgsLoader) LoadPackage(modroot string, path, name string) error {
	// Initialize a queue with the root package
	queue := []gnomod.Pkg{{Dir: path, Name: name}}

	for len(queue) > 0 {
		// Dequeue the first package
		currentPkg := queue[0]
		queue = queue[1:]

		if currentPkg.Dir == "" {
			return fmt.Errorf("no path specified for package")
		}

		if currentPkg.Name == "" {
			// Load `gno.mod` information
			gnoModPath := filepath.Join(currentPkg.Dir, "gno.mod")
			gm, err := gnomod.ParseGnoMod(gnoModPath)
			if err != nil {
				return fmt.Errorf("unable to load %q: %w", gnoModPath, err)
			}
			gm.Sanitize()

			// Override package info with mod infos
			currentPkg.Name = gm.Module.Mod.Path
			currentPkg.Draft = gm.Draft
			for _, req := range gm.Require {
				currentPkg.Requires = append(currentPkg.Requires, req.Mod.Path)
			}
		}

		if currentPkg.Draft {
			continue // Skip draft package
		}

		if pl.exist(currentPkg) {
			continue
		}
		pl.add(currentPkg)

		// Add requirements to the queue
		for _, pkgPath := range currentPkg.Requires {
			fullPath := filepath.Join(modroot, pkgPath)
			queue = append(queue, gnomod.Pkg{Dir: fullPath})
		}
	}

	return nil
}

func (pl *pkgsLoader) add(pkg gnomod.Pkg) {
	pl.visited[pkg.Name] = struct{}{}
	pl.pkgs = append(pl.pkgs, pkg)
}

func (pl *pkgsLoader) exist(pkg gnomod.Pkg) (ok bool) {
	_, ok = pl.visited[pkg.Name]
	return
}
