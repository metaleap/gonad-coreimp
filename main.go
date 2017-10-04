package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-forks/pflag"
	"github.com/metaleap/go-util-fs"
	"github.com/metaleap/go-util-misc"
)

var (
	Proj      BowerProject
	Deps      = map[string]*BowerProject{}
	mapsmutex sync.Mutex
	Flag      struct {
		NoPrefix      bool
		Comments      bool
		ForceRegenAll bool
		GoDirSrcPath  string
		GoNamespace   string
	}
)

func main() {
	var wg sync.WaitGroup
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
	err := ufs.EnsureDirExists(Flag.GoDirSrcPath)
	if err == nil {
		if !ufs.DirExists(Proj.DepsDirPath) {
			panic("No such `dependency-path` directory: " + Proj.DepsDirPath)
		}
		if !ufs.DirExists(Proj.SrcDirPath) {
			panic("No such `src-path` directory: " + Proj.SrcDirPath)
		}
		if err = Proj.LoadFromJsonFile(false); err == nil {
			checkifdepdirhasbowerfile := func(reldirpath string) {
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
			ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
				wg.Add(1)
				go checkifdepdirhasbowerfile(reldirpath)
				return true
			})
			wg.Wait()
			loaddepfrombowerfile := func(depname string, dep *BowerProject) {
				defer wg.Done()
				if e := dep.LoadFromJsonFile(true); e != nil {
					panic(e)
				}
			}
			for dk, dv := range Deps {
				wg.Add(1)
				go loaddepfrombowerfile(dk, dv)
			}
			if wg.Wait(); err == nil {
				loadgirmetas := func(dep *BowerProject) {
					defer wg.Done()
					dep.EnsureModPkgGirMetas()
				}
				for _, dep := range Deps {
					wg.Add(1)
					go loadgirmetas(dep)
				}
				wg.Add(1)
				go loadgirmetas(&Proj)
				if err = Proj.EnsureOutDirs(); err == nil {
					for _, dep := range Deps {
						if err = dep.EnsureOutDirs(); err != nil {
							break
						}
					}
				}
				wg.Wait()
				if err == nil {
					regengirasts := func(dep *BowerProject) {
						defer wg.Done()
						dep.RegenModPkgGirAsts()
						dep.WriteOutDirtyGirMetas()
					}
					for _, dep := range Deps {
						wg.Add(1)
						go regengirasts(dep)
					}
					wg.Add(1)
					go regengirasts(&Proj)
					wg.Wait()
				}
			}
		}
	}
	if err != nil {
		panic(err.Error())
	}
}
