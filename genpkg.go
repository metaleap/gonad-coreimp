package main

import (
	"path/filepath"

	"github.com/metaleap/go-util-fs"
)

type ModuleInfo struct {
	regenerate    bool
	qName         string //	eg	Control.Monad.Eff.Uncurried
	lName         string //	eg	Uncurried
	srcFilePath   string //	eg	bower_components/purescript-eff/src/Control/Monad/Eff/Uncurried.purs
	impFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/coreimp.json
	extFilePath   string //	eg	output/Control.Monad.Eff.Uncurried/externs.json
	goOutFilePath string //	eg	Control/Monad/Eff/Uncurried/Uncurried.go
}

func (me *BowerProject) RegeneratePkgs() {
	gopkgdir := filepath.Join(Flag.GoDirSrcPath, me.GoOut.PkgDirPath)
	for _, modinfo := range me.Modules {
		gopkgfile := filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if ufs.DirExists(gopkgdir) {
			println("OK!\t" + gopkgfile)
		} else {
			println("UH!\t" + gopkgfile)
		}
		// ufs.WriteTextFile("filePath", "contents")
	}
}
