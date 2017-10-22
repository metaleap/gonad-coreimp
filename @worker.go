package main

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util-fs"
)

type worker struct {
	sync.WaitGroup
}

func (me *worker) gø(fn func(*psBowerProject)) {
	for _, d := range Deps {
		me.Add(1)
		go fn(d)
	}
	me.Wait()
}

func (me *worker) checkIfDepDirHasBowerFile(locker sync.Locker, reldirpath string) {
	defer me.Done()
	jsonfilepath := filepath.Join(reldirpath, ".bower.json")
	if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
		bproj := &psBowerProject{
			DepsDirPath: Proj.DepsDirPath, JsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
		}
		defer locker.Unlock()
		locker.Lock()
		Deps[depname] = bproj
	}
}

func (me *worker) loadDepFromBowerFile(dep *psBowerProject) {
	defer me.Done()
	if err := dep.loadFromJsonFile(); err != nil {
		panic(err)
	}
}

func (me *worker) loadIrMetas(dep *psBowerProject) {
	defer me.Done()
	dep.ensureModPkgIrMetas()
}

func (me *worker) populateIrMetas(dep *psBowerProject) {
	defer me.Done()
	dep.populateModPkgIrMetas()
}

func (me *worker) prepIrAsts(dep *psBowerProject) {
	defer me.Done()
	dep.prepModPkgIrAsts()
	if err := dep.writeOutDirtyIrMetas(false); err != nil {
		panic(err)
	}
}

func (me *worker) reGenIrAsts(dep *psBowerProject) {
	defer me.Done()
	dep.reGenModPkgIrAsts()
	if err := dep.writeOutDirtyIrMetas(true); err != nil {
		panic(err)
	}
}
