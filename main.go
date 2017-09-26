package main

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-forks/pflag"
	"github.com/metaleap/go-util-fs"
	"github.com/metaleap/go-util-misc"
)

var (
	Proj BowerProject
	Deps = map[string]BowerProject{}
	Flag struct {
		NoPrefix     bool
		Comments     bool
		GoDirSrcPath string
		GoNamespace  string
	}
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	pflag.StringVar(&Proj.SrcDirPath, "src-path", "src", "Project-sources directory path")
	pflag.StringVar(&Proj.DepsDirPath, "dependency-path", "bower_components", "Dependencies directory path")
	pflag.StringVar(&Proj.JsonFilePath, "bower-file", "bower.json", "Project file path")
	pflag.StringVar(&Proj.DumpsDirProjPath, "coreimp-dumps-path", "output", "Directory path of 'purs' per-module output directories")
	pflag.BoolVar(&Flag.NoPrefix, "no-prefix", false, "Do not include comment header")
	pflag.BoolVar(&Flag.Comments, "comments", false, "Include comments in the generated code")
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
			ufs.WalkDirsIn(Proj.DepsDirPath, func(reldirpath string) bool {
				jsonfilepath := filepath.Join(reldirpath, ".bower.json")
				if !ufs.FileExists(jsonfilepath) {
					jsonfilepath = filepath.Join(reldirpath, "bower.json")
				}
				if depname := strings.TrimLeft(reldirpath[len(Proj.DepsDirPath):], "\\/"); ufs.FileExists(jsonfilepath) {
					Deps[depname] = BowerProject{
						DepsDirPath: Proj.DepsDirPath, JsonFilePath: jsonfilepath, SrcDirPath: filepath.Join(reldirpath, "src"),
					}
				}
				return true
			})
			for dk, dv := range Deps {
				println(dk + "\t" + dv.JsonFilePath + "\t" + dv.SrcDirPath + "\t")
				if err = dv.LoadFromJsonFile(true); err != nil {
					break
				}
			}
			if err == nil {
				panic(Proj.JsonFile.Name)
			}
		}
	}
	if err != nil {
		panic(err.Error())
	}
}
