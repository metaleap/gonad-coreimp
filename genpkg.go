package main

import (
	"path/filepath"
	"strings"

	"github.com/metaleap/go-util-fs"
)

type ModuleInfo struct {
	regenerate    bool
	qName         string //	eg	Control.Monad.Eff.Uncurried
	lName         string //	eg	Uncurried
	pName         string //	eg	ControlMonadEffUncurried
	srcFilePath   string //	eg	bower_components/purescript-eff/src/Control/Monad/Eff/Uncurried.purs
	impFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/coreimp.json
	extFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/externs.json
	goOutFilePath string //	eg	Control/Monad/Eff/Uncurried/Uncurried.go
}

var (
	slash2dot      = strings.NewReplacer("\\", ".", "/", ".")
	dot2underscore = strings.NewReplacer(".", "_")
)

func (me *BowerProject) AddModuleInfoFromPursFileIfCoreimp(relpath string, gopkgdir string) {
	i, l := strings.LastIndexAny(relpath, "/\\"), len(relpath)-5
	modinfo := &ModuleInfo{
		srcFilePath: filepath.Join(me.SrcDirPath, relpath),
		qName:       slash2dot.Replace(relpath[:l]), lName: relpath[i+1 : l],
	}
	if modinfo.impFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "coreimp.json"); ufs.FileExists(modinfo.impFilePath) {
		modinfo.pName = dot2underscore.Replace(modinfo.qName)
		modinfo.extFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "externs.json")
		modinfo.goOutFilePath = filepath.Join(relpath[:l], modinfo.lName) + ".go"
		gopkgfile := filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if !ufs.FileExists(gopkgfile) {
			modinfo.regenerate = true
		} else if ufs.FileExists(modinfo.impFilePath) {
			modinfo.regenerate, _ = ufs.IsNewerThan(modinfo.impFilePath, gopkgfile)
		}
		me.Modules = append(me.Modules, modinfo)
	}
}

func (me *BowerProject) RegeneratePkgs() (err error) {
	gopkgdir := filepath.Join(Flag.GoDirSrcPath, me.GoOut.PkgDirPath)
	for _, modinfo := range me.Modules {
		if modinfo.regenerate || Flag.ForceRegenAll {
			gopkgfile := filepath.Join(gopkgdir, modinfo.goOutFilePath)
			if err = ufs.EnsureDirExists(filepath.Dir(gopkgfile)); err != nil {
				return
			}
			if err = ufs.WriteTextFile(gopkgfile, "package "+modinfo.pName); err != nil {
				return
			}
			println(gopkgfile)
		}
	}
	return
}
