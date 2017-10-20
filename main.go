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
)

var (
	Proj PsBowerProject
	Deps = map[string]*PsBowerProject{}
	Flag struct {
		NoPrefix      bool
		Comments      bool
		ForceRegenAll bool
		DumpAst       bool
		GoDirSrcPath  string
		GoNamespace   string
	}
)

func checkIfDepDirHasBowerFile(wg *sync.WaitGroup, mutex *sync.Mutex, reldirpath string) {
	defer wg.Done()
	jsonfilepath := filepath.Join(reldirpath, ".bower.json")
	if !ufs.FileExists(jsonfilepath) {
		jsonfilepath = filepath.Join(reldirpath, "bower.json")
	}
	if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
		bproj := &PsBowerProject{
			DepsDirPath: Proj.DepsDirPath, JsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
		}
		defer mutex.Unlock()
		mutex.Lock()
		Deps[depname] = bproj
	}
}

func loadDepFromBowerFile(wg *sync.WaitGroup, depname string, dep *PsBowerProject) {
	defer wg.Done()
	if err := dep.LoadFromJsonFile(true); err != nil {
		panic(err)
	}
}

func loadGIrMetas(wg *sync.WaitGroup, dep *PsBowerProject) {
	defer wg.Done()
	dep.EnsureModPkgGIrMetas()
}

func prepGIrAsts(wg *sync.WaitGroup, dep *PsBowerProject) {
	defer wg.Done()
	dep.PrepModPkgGIrAsts()
	if err := dep.WriteOutDirtyGIrMetas(false); err != nil {
		panic(err)
	}
}

func reGenGIrAsts(wg *sync.WaitGroup, dep *PsBowerProject) {
	defer wg.Done()
	dep.ReGenModPkgGIrAsts()
	if err := dep.WriteOutDirtyGIrMetas(true); err != nil {
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
		if err = Proj.LoadFromJsonFile(false); err == nil {
			var mutex sync.Mutex
			var wg sync.WaitGroup
			ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
				wg.Add(1)
				go checkIfDepDirHasBowerFile(&wg, &mutex, reldirpath)
				return true
			})
			wg.Wait()
			for dk, dv := range Deps {
				wg.Add(1)
				go loadDepFromBowerFile(&wg, dk, dv)
			}
			if wg.Wait(); err == nil {
				for _, dep := range Deps {
					wg.Add(1)
					go loadGIrMetas(&wg, dep)
				}
				wg.Add(1)
				go loadGIrMetas(&wg, &Proj)
				if err = Proj.EnsureOutDirs(); err == nil {
					for _, dep := range Deps {
						if err = dep.EnsureOutDirs(); err != nil {
							break
						}
					}
				}
				wg.Wait()
				if err == nil {
					for _, dep := range Deps {
						wg.Add(1)
						go prepGIrAsts(&wg, dep)
					}
					wg.Add(1)
					go prepGIrAsts(&wg, &Proj)
					wg.Wait()
					if err == nil {
						for _, dep := range Deps {
							wg.Add(1)
							go reGenGIrAsts(&wg, dep)
						}
						wg.Add(1)
						go reGenGIrAsts(&wg, &Proj)
						allpkgimppaths := map[string]bool{}
						numregen := countNumOfReGendModules(allpkgimppaths) // do this even when ForceRegenAll to have the map filled
						if Flag.ForceRegenAll {
							numregen = len(allpkgimppaths)

						}
						if wg.Wait(); err == nil {
							dur := time.Now().Sub(starttime)
							if fmt.Printf("Processing %d modules (re-generating %d) took me %v\n", len(allpkgimppaths), numregen, dur); numregen > 0 {
								fmt.Printf("\t(avg. %v per re-generated module)\n", dur/time.Duration(numregen))
							}
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
	for _, mod := range Proj.Modules {
		if allpkgimppaths[path.Join(Proj.GoOut.PkgDirPath, mod.goOutDirPath)] = mod.reGenGIr; mod.reGenGIr {
			numregen++
		}
	}
	for _, dep := range Deps {
		for _, mod := range dep.Modules {
			if allpkgimppaths[path.Join(dep.GoOut.PkgDirPath, mod.goOutDirPath)] = mod.reGenGIr; mod.reGenGIr {
				numregen++
			}
		}
	}
	return
}

func writeTestMainGo(allpkgimppaths map[string]bool) (err error) {
	w := &bytes.Buffer{}
	if _, err = fmt.Fprintln(w, "package main\n\nimport ("); err == nil {
		//	we sort them to avoid useless diffs
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
	}
	return
}
