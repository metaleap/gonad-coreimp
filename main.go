package main

import (
	"github.com/go-forks/pflag"
)

var (
	Proj BowerProject
	Flag struct {
		NoPrefix bool
	}
)

func main() {
	pflag.StringVar(&Proj.DirRootPath, "dir", ".", "Project directory path")
	pflag.StringVar(&Proj.DepsDirRelPath, "dir-src", "src", "Project sources directory path (relative to --dir)")
	pflag.StringVar(&Proj.DepsDirRelPath, "dir-deps", "bower_components", "Dependencies directory path (relative to --dir)")
	pflag.StringVar(&Proj.JsonFileRelPath, "projfile", "bower.json", "Project file path (relative to --dir)")
	// pflag.BoolVar(p, name, value, usage)
	pflag.Parse()
}
