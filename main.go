package main

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-forks/pflag"
	"github.com/metaleap/go-util-fs"
	"github.com/metaleap/go-util-misc"
)

var (
	Proj BowerProject
	Deps = map[string]*BowerProject{}
	Flag struct {
		NoPrefix      bool
		Comments      bool
		ForceRegenAll bool
		GoDirSrcPath  string
		GoNamespace   string
	}

	err            error
	mapsmutex      sync.Mutex
	wg             sync.WaitGroup
	allpkgimppaths = map[string]bool{}
)

func checkIfDepDirHasBowerFile(reldirpath string) {
	defer wg.Done()
	jsonfilepath := filepath.Join(reldirpath, ".bower.json")
	if !ufs.FileExists(jsonfilepath) {
		jsonfilepath = filepath.Join(reldirpath, "bower.json")
	}
	if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
		bproj := &BowerProject{
			DepsDirPath: Proj.DepsDirPath, JsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
		}
		defer mapsmutex.Unlock()
		mapsmutex.Lock()
		Deps[depname] = bproj
	}
}

func loadDepFromBowerFile(depname string, dep *BowerProject) {
	defer wg.Done()
	if err = dep.LoadFromJsonFile(true); err != nil {
		panic(err)
	}
}

func loadGIrMetas(dep *BowerProject) {
	defer wg.Done()
	dep.EnsureModPkgGIrMetas()
}

func prepGIrAsts(dep *BowerProject) {
	defer wg.Done()
	dep.PrepOrReGenModPkgGIrAsts(true)
	if err = dep.WriteOutDirtyGIrMetas(false); err != nil {
		panic(err)
	}
}

func reGenGIrAsts(dep *BowerProject) {
	defer wg.Done()
	dep.PrepOrReGenModPkgGIrAsts(false)
	if err = dep.WriteOutDirtyGIrMetas(true); err != nil {
		panic(err)
	}
}

func main() {
	starttime := time.Now()
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	pflag.StringVar(&Proj.SrcDirPath, "src-path", "src", "Project-sources directory path")
	pflag.StringVar(&Proj.DepsDirPath, "dependency-path", "bower_components", "Dependencies directory path")
	pflag.StringVar(&Proj.JsonFilePath, "bower-file", "bower.json", "Project file path")
	pflag.StringVar(&Proj.DumpsDirProjPath, "coreimp-dumps-path", "output", "Directory path of 'purs' per-module output directories")
	pflag.BoolVar(&Flag.NoPrefix, "no-prefix", false, "Do not include comment header")
	pflag.BoolVar(&Flag.Comments, "comments", false, "Include comments in the generated code")
	pflag.BoolVar(&Flag.ForceRegenAll, "force", false, "Force re-generating all applicable (coreimp dumps present) packages")
	for _, gopath := range ugo.GoPaths() {
		if Flag.GoDirSrcPath = filepath.Join(gopath, "src"); ufs.DirExists(Flag.GoDirSrcPath) {
			break
		}
	}
	pflag.StringVar(&Flag.GoDirSrcPath, "build-path", Flag.GoDirSrcPath, "The output GOPATH for generated Go packages")
	Flag.GoNamespace = filepath.Join("github.com", "gonadz")
	pflag.StringVar(&Flag.GoNamespace, "go-namespace", Flag.GoNamespace, "Root namespace for all generated Go packages")
	pflag.Parse()
	if err = ufs.EnsureDirExists(Flag.GoDirSrcPath); err == nil {
		if !ufs.DirExists(Proj.DepsDirPath) {
			panic("No such `dependency-path` directory: " + Proj.DepsDirPath)
		}
		if !ufs.DirExists(Proj.SrcDirPath) {
			panic("No such `src-path` directory: " + Proj.SrcDirPath)
		}
		if err = Proj.LoadFromJsonFile(false); err == nil {
			ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
				wg.Add(1)
				go checkIfDepDirHasBowerFile(reldirpath)
				return true
			})
			wg.Wait()
			for dk, dv := range Deps {
				wg.Add(1)
				go loadDepFromBowerFile(dk, dv)
			}
			if wg.Wait(); err == nil {
				for _, dep := range Deps {
					wg.Add(1)
					go loadGIrMetas(dep)
				}
				wg.Add(1)
				go loadGIrMetas(&Proj)
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
						go prepGIrAsts(dep)
					}
					wg.Add(1)
					go prepGIrAsts(&Proj)
					wg.Wait()
					if err == nil {
						for _, dep := range Deps {
							wg.Add(1)
							go reGenGIrAsts(dep)
						}
						wg.Add(1)
						go reGenGIrAsts(&Proj)
						numregen := 0
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
						if Flag.ForceRegenAll {
							numregen = len(allpkgimppaths)
						}
						if wg.Wait(); err == nil {
							dur := time.Now().Sub(starttime)
							if fmt.Printf("Processing %d modules (re-generating %d) took me %v\n", len(allpkgimppaths), numregen, dur); numregen > 0 {
								fmt.Printf("\t(avg. %v per re-generated module)\n", dur/time.Duration(numregen))
							}
							err = writeTestMainGo()
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

func writeTestMainGo() (err error) {
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
