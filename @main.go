package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-forks/pflag"
	"github.com/metaleap/go-util/fs"
	"github.com/metaleap/go-util/slice"
)

var (
	Proj psBowerProject
	Deps = map[string]*psBowerProject{}
	Flag struct {
		ForceAll bool
		NoPrefix bool
		Comments bool
	}
)

func main() {
	debug.SetGCPercent(-1) // we're (hopefully) not a long-running process
	starttime := time.Now()
	// args match those of purs and/or pulp where there's overlap, other config goes in bower.json's `Gonad` field (see `psBowerFile`)
	pflag.StringVar(&Proj.SrcDirPath, "src-path", "src", "Project-sources directory path")
	pflag.StringVar(&Proj.DepsDirPath, "dependency-path", "bower_components", "Dependencies directory path")
	pflag.StringVar(&Proj.BowerJsonFilePath, "bower-file", "bower.json", "Project file path (further configuration options possible in the Gonad field)")
	pflag.BoolVar(&Flag.NoPrefix, "no-prefix", false, "Do not include comment header")
	pflag.BoolVar(&Flag.Comments, "comments", false, "Include comments in the generated code")
	pflag.BoolVar(&Flag.ForceAll, "force", false, "Force-regenerate all *.go & *.json files, not just the outdated or missing ones")
	pflag.Parse()
	var err error
	if !ufs.DirExists(Proj.DepsDirPath) {
		err = fmt.Errorf("No such `dependency-path` directory: %s", Proj.DepsDirPath)
	} else if !ufs.DirExists(Proj.SrcDirPath) {
		err = fmt.Errorf("No such `src-path` directory: %s", Proj.SrcDirPath)
	} else if err = Proj.loadFromJsonFile(); err == nil {
		var do mainWorker
		var mutex sync.Mutex
		ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
			do.Add(1)
			go do.checkIfDepDirHasBowerFile(&mutex, reldirpath)
			return true
		})
		do.Wait()
		do.forAllDeps(do.loadDepFromBowerFile)
		Deps[""] = &Proj // from now on, all Deps and the main Proj are handled in parallel and equivalently
		do.forAllDeps(do.loadIrMetas)
		for _, dep := range Deps {
			if err = dep.ensureOutDirs(); err != nil {
				break
			}
		}
		if err == nil {
			do.forAllDeps(do.populateIrMetas)
			do.forAllDeps(do.prepIrAsts)
			do.forAllDeps(do.reGenIrAsts)
			do.forAllDeps(do.writeOutFiles)
			dur := time.Since(starttime)
			allpkgimppaths := map[string]bool{}
			numregen, numtotal := countNumOfReGendModules(allpkgimppaths) // do this even when ForceAll to have the map filled for writeTestMainGo
			if Flag.ForceAll {
				numregen = numtotal
			}
			if Proj.BowerJsonFile.Gonad.Out.MainDepLevel > 0 {
				err = writeTestMainGo(allpkgimppaths)
			}
			if err == nil {
				fmt.Printf("Processing %d modules (re-generating %d) took me %v\n", numtotal, numregen, dur)
			}
		}
	}
	if err != nil {
		panic(err.Error())
	}
}

func countNumOfReGendModules(allpkgimppaths map[string]bool) (numregen int, numtotal int) {
	for _, dep := range Deps {
		for _, mod := range dep.Modules {
			if allpkgimppaths[mod.impPath()], numtotal = mod.reGenIr, numtotal+1; mod.reGenIr {
				numregen++
			}
		}
	}
	return
}

func writeTestMainGo(allpkgimppaths map[string]bool) (err error) {
	w := &bytes.Buffer{}
	fmt.Fprintln(w, "package main\n\nimport (")

	// temporary commandline option to only import a sub-set of packages
	okpkgs := []string{}
	for i := 0; i < Proj.BowerJsonFile.Gonad.Out.MainDepLevel; i++ {
		thisok := []string{}
		for _, dep := range Deps {
			for _, mod := range dep.Modules {
				if modimppath := mod.impPath(); !uslice.StrHas(okpkgs, modimppath) {
					isthisok := true
					for _, imp := range mod.irMeta.Imports {
						if imp.emitted && !uslice.StrHas(okpkgs, imp.ImpPath) {
							if !(imp.PsModQName == "" || strings.HasPrefix(imp.ImpPath, prefixDefaultFfiPkgImpPath)) {
								isthisok = false
								break
							}
						}
					}
					if isthisok {
						fmt.Printf("dep level #%d\t%s\n", i+1, modimppath)
						thisok = append(thisok, modimppath)
					}
				}
			}
		}
		okpkgs = append(okpkgs, thisok...)
	}
	for pkgimppath, _ := range allpkgimppaths {
		if !uslice.StrHas(okpkgs, pkgimppath) {
			delete(allpkgimppaths, pkgimppath)
		}
	}

	//	we sort them
	pkgimppaths := sort.StringSlice{}
	for pkgimppath, _ := range allpkgimppaths {
		pkgimppaths = append(pkgimppaths, pkgimppath)
	}
	sort.Strings(pkgimppaths)
	for _, pkgimppath := range pkgimppaths {
		if _, err = fmt.Fprintf(w, "\t_ %q\n", pkgimppath); err != nil {
			return
		}
	}
	if _, err = fmt.Fprintln(w, ")\n\nfunc main() { println(\"Looks like this compiled just fine!\") }"); err == nil {
		err = ufs.WriteTextFile(filepath.Join(Proj.BowerJsonFile.Gonad.Out.GoDirSrcPath, Proj.GoOut.PkgDirPath, "check-if-all-gonad-generated-packages-compile.go"), w.String())
	}
	return
}
