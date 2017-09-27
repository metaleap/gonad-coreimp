package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util-fs"
)

type ModuleInfo struct {
	regenerate    bool
	qName         string //	eg	Control.Monad.Eff.Uncurried
	lName         string //	eg	Uncurried
	pName         string //	eg	Control_Monad_Eff_Uncurried
	srcFilePath   string //	eg	bower_components/purescript-eff/src/Control/Monad/Eff/Uncurried.purs
	impFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/coreimp.json
	extFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/externs.json
	goOutFilePath string //	eg	Control/Monad/Eff/Uncurried/Uncurried.go

	gopkgfilepath string //	full target file path (not necessarily absolute but starting with the applicable gopath)
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
		modinfo.gopkgfilepath = filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if !ufs.FileExists(modinfo.gopkgfilepath) {
			modinfo.regenerate = true
		} else if ufs.FileExists(modinfo.impFilePath) {
			modinfo.regenerate, _ = ufs.IsNewerThan(modinfo.impFilePath, modinfo.gopkgfilepath)
		}
		me.Modules = append(me.Modules, modinfo)
	}
}

func (me *BowerProject) RegenerateModulePkgs() (err error) {
	for _, modinfo := range me.Modules {
		if modinfo.regenerate || Flag.ForceRegenAll {
			if err = ufs.EnsureDirExists(filepath.Dir(modinfo.gopkgfilepath)); err != nil {
				return
			}
		}
	}
	var wg sync.WaitGroup
	regenmodulepkg := func(modinfo *ModuleInfo) {
		defer wg.Done()
		if e := modinfo.regenPkg(me); e != nil {
			panic(e)
		}
	}
	for _, modinfo := range me.Modules {
		if modinfo.regenerate || Flag.ForceRegenAll {
			wg.Add(1)
			go regenmodulepkg(modinfo)
		}
	}
	wg.Wait()
	return
}

func (me *ModuleInfo) regenPkg(proj *BowerProject) (err error) {
	var buf bytes.Buffer
	if _, err = buf.WriteString("package " + me.pName); err == nil {
		err = ufs.WriteBinaryFile(me.gopkgfilepath, buf.Bytes())
	}
	return
}
