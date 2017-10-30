package main

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util/fs"
)

type mainWorker struct {
	sync.WaitGroup
}

func (me *mainWorker) forAllDeps(fn func(*psBowerProject)) {
	for _, d := range Deps {
		me.Add(1)
		go fn(d)
	}
	me.Wait()
}

func (me *mainWorker) checkIfDepDirHasBowerFile(locker sync.Locker, reldirpath string) {
	defer me.Done()
	jsonfilepath := filepath.Join(reldirpath, ".bower.json")
	if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
		bproj := &psBowerProject{
			DepsDirPath: Proj.DepsDirPath, BowerJsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
		}
		defer locker.Unlock()
		locker.Lock()
		Deps[depname] = bproj
	}
}

func (me *mainWorker) loadDepFromBowerFile(dep *psBowerProject) {
	defer me.Done()
	if err := dep.loadFromJsonFile(); err != nil {
		panic(err)
	}
}

func (me *mainWorker) loadIrMetas(dep *psBowerProject) {
	defer me.Done()
	dep.ensureModPkgIrMetas()
}

func (me *mainWorker) populateIrMetas(dep *psBowerProject) {
	defer me.Done()
	dep.populateModPkgIrMetas()
}

func (me *mainWorker) prepIrAsts(dep *psBowerProject) {
	defer me.Done()
	dep.prepModPkirAsts()
}

func (me *mainWorker) reGenIrAsts(dep *psBowerProject) {
	defer me.Done()
	dep.reGenModPkirAsts()
}

func (me *mainWorker) writeOutFiles(dep *psBowerProject) {
	defer me.Done()
	if err := dep.writeOutFiles(); err != nil {
		panic(err)
	}
}
