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

func checkIfDepDirHasBowerFile(wg *sync.WaitGroup, mutex sync.Locker, reldirpath string) {
	defer wg.Done()
	jsonfilepath := filepath.Join(reldirpath, ".bower.json")
	if !ufs.FileExists(jsonfilepath) {
		jsonfilepath = filepath.Join(reldirpath, "bower.json")
	}
	if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
		bproj := &psBowerProject{
			DepsDirPath: Proj.DepsDirPath, JsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
		}
		defer mutex.Unlock()
		mutex.Lock()
		Deps[depname] = bproj
	}
}

func loadDepFromBowerFile(wg *sync.WaitGroup, dep *psBowerProject) {
	defer wg.Done()
	if err := dep.loadFromJsonFile(); err != nil {
		panic(err)
	}
}

func loadIrMetas(wg *sync.WaitGroup, dep *psBowerProject) {
	defer wg.Done()
	dep.ensureModPkgIrMetas()
}

func populateIrMetas(wg *sync.WaitGroup, dep *psBowerProject) {
	defer wg.Done()
	dep.populateModPkgIrMetas()
}

func prepIrAsts(wg *sync.WaitGroup, dep *psBowerProject) {
	defer wg.Done()
	dep.prepModPkgIrAsts()
	if err := dep.writeOutDirtyIrMetas(false); err != nil {
		panic(err)
	}
}

func reGenIrAsts(wg *sync.WaitGroup, dep *psBowerProject) {
	defer wg.Done()
	dep.reGenModPkgIrAsts()
	if err := dep.writeOutDirtyIrMetas(true); err != nil {
		panic(err)
	}
}

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
			var mutex sync.Mutex
			var wg sync.WaitGroup
			ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
				wg.Add(1)
				go checkIfDepDirHasBowerFile(&wg, &mutex, reldirpath)
				return true
			})
			wg.Wait()
			for _, dv := range Deps {
				wg.Add(1)
				go loadDepFromBowerFile(&wg, dv)
			}
			if wg.Wait(); err == nil {
				Deps[""] = &Proj // from now on, all deps and the main proj are handled in parallel and equivalently
				for _, dep := range Deps {
					wg.Add(1)
					go loadIrMetas(&wg, dep)
				}
				for _, dep := range Deps {
					if err = dep.ensureOutDirs(); err != nil {
						break
					}
				}
				if wg.Wait(); err == nil {
					for _, dep := range Deps {
						wg.Add(1)
						go populateIrMetas(&wg, dep)
					}
					wg.Wait()
					for _, dep := range Deps {
						wg.Add(1)
						go prepIrAsts(&wg, dep)
					}
					wg.Wait()
					if err == nil {
						for _, dep := range Deps {
							wg.Add(1)
							go reGenIrAsts(&wg, dep)
						}
						allpkgimppaths := map[string]bool{}
						numregen := countNumOfReGendModules(allpkgimppaths) // do this even when ForceRegenAll to have the map filled
						if Flag.ForceRegenAll {
							numregen = len(allpkgimppaths)
						}
						if wg.Wait(); err == nil {
							dur := time.Since(starttime)
							fmt.Printf("Processing %d modules (re-generating %d) took me %v\n", len(allpkgimppaths), numregen, dur)
							err = writeTestMainGo(allpkgimppaths)
						}
					}
				}
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
