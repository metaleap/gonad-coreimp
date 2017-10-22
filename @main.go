package main

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-forks/pflag"
	"github.com/metaleap/go-util-fs"
	"github.com/metaleap/go-util-misc"
	"github.com/metaleap/go-util-slice"
)

var (
	Proj psBowerProject
	Deps = map[string]*psBowerProject{}
	Flag struct {
		NoPrefix      bool
		Comments      bool
		ForceRegenAll bool
		DumpAst       bool
		GoDirSrcPath  string
		GoNamespace   string
		DepLevel      int
	}
)

func main() {
	// runtime.SetGCPercent(-1) // turn off GC, we're a quickly-in-and-out-again program
	starttime := time.Now()
	// args partially match those of purs and/or pulp where there's overlap
	pflag.StringVar(&Proj.SrcDirPath, "src-path", "src", "Project-sources directory path")
	pflag.StringVar(&Proj.DepsDirPath, "dependency-path", "bower_components", "Dependencies directory path")
	pflag.StringVar(&Proj.JsonFilePath, "bower-file", "bower.json", "Project file path")
	pflag.StringVar(&Proj.DumpsDirProjPath, "coreimp-dumps-path", "output", "Directory path of 'purs' per-module output directories")
	pflag.BoolVar(&Flag.NoPrefix, "no-prefix", false, "Do not include comment header")
	pflag.BoolVar(&Flag.Comments, "comments", false, "Include comments in the generated code")
	pflag.BoolVar(&Flag.ForceRegenAll, "force", false, "Force re-generating all applicable (coreimp dumps present) packages")
	pflag.BoolVar(&Flag.DumpAst, "dump-ast", false, "Dumps a gonad.ast.json next to gonad.json")
	pflag.IntVar(&Flag.DepLevel, "dep-level", -1, "(temporary option)")
	for _, gopath := range ugo.GoPaths() {
		if Flag.GoDirSrcPath = filepath.Join(gopath, "src"); ufs.DirExists(Flag.GoDirSrcPath) {
			break
		}
	}
	pflag.StringVar(&Flag.GoDirSrcPath, "build-path", Flag.GoDirSrcPath, "The output GOPATH for generated Go packages")
	Flag.GoNamespace = filepath.Join("github.com", "gonadz")
	pflag.StringVar(&Flag.GoNamespace, "go-namespace", Flag.GoNamespace, "Root namespace for all generated Go packages")
	pflag.Parse()
	var err error
	if err = ufs.EnsureDirExists(Flag.GoDirSrcPath); err == nil {
		if !ufs.DirExists(Proj.DepsDirPath) {
			panic("No such `dependency-path` directory: " + Proj.DepsDirPath)
		}
		if !ufs.DirExists(Proj.SrcDirPath) {
			panic("No such `src-path` directory: " + Proj.SrcDirPath)
		}
		if err = Proj.loadFromJsonFile(); err == nil {
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
				allpkgimppaths := map[string]bool{}
				numregen := countNumOfReGendModules(allpkgimppaths) // do this even when ForceRegenAll to have the map filled
				if Flag.ForceRegenAll {
					numregen = len(allpkgimppaths)
				}
				dur := time.Since(starttime)
				fmt.Printf("Processing %d modules (re-generating %d) took me %v\n", len(allpkgimppaths), numregen, dur)
				err = writeTestMainGo(allpkgimppaths)
			}
		}
	}
	if err != nil {
		panic(err.Error())
	}
}

func countNumOfReGendModules(allpkgimppaths map[string]bool) (numregen int) {
	for _, dep := range Deps {
		for _, mod := range dep.Modules {
			if allpkgimppaths[path.Join(dep.GoOut.PkgDirPath, mod.goOutDirPath)] = mod.reGenIr; mod.reGenIr {
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
	if Flag.DepLevel >= 0 {
		okpkgs := []string{}
		for i := 0; i <= Flag.DepLevel; i++ {
			thisok := []string{}
			for _, dep := range Deps {
				for _, mod := range dep.Modules {
					if modimppath := path.Join(dep.GoOut.PkgDirPath, mod.goOutDirPath); !uslice.StrHas(okpkgs, modimppath) {
						isthisok := true
						for _, imp := range mod.irMeta.Imports {
							if (!uslice.StrHas(okpkgs, imp.P)) && !strings.Contains(imp.Q, nsPrefixDefaultFfiPkg) {
								isthisok = false
								break
							}
						}
						if isthisok {
							println(modimppath)
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
		err = ufs.WriteTextFile(filepath.Join(Flag.GoDirSrcPath, Proj.GoOut.PkgDirPath, "check-if-all-gonad-generated-packages-compile.go"), w.String())
	}
	return
}
