package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util-dev/bower"
	"github.com/metaleap/go-util-fs"
)

/*
Represents either the given PureScript main `src` project
or one of its dependency libs usually found in `bower_components`.
*/

type psBowerProject struct {
	JsonFilePath     string
	SrcDirPath       string
	DepsDirPath      string
	DumpsDirProjPath string
	JsonFile         udevbower.BowerFile
	Modules          []*modPkg
	GoOut            struct {
		PkgDirPath string
	}
}

func (me *psBowerProject) ensureOutDirs() (err error) {
	dirpath := filepath.Join(Flag.GoDirSrcPath, me.GoOut.PkgDirPath)
	if err = ufs.EnsureDirExists(dirpath); err == nil {
		for _, depmod := range me.Modules {
			if err = ufs.EnsureDirExists(filepath.Join(dirpath, depmod.goOutDirPath)); err != nil {
				break
			}
		}
	}
	return
}

func (me *psBowerProject) moduleByQName(qname string) *modPkg {
	for _, m := range me.Modules {
		if m.qName == qname {
			return m
		}
	}
	return nil
}

func (me *psBowerProject) moduleByPName(pname string) *modPkg {
	for _, m := range me.Modules {
		if m.pName == pname {
			return m
		}
	}
	return nil
}

func (me *psBowerProject) loadFromJsonFile(isdep bool) (err error) {
	if err = udevbower.LoadFromFile(me.JsonFilePath, &me.JsonFile); err == nil {
		me.GoOut.PkgDirPath = Flag.GoNamespace
		if repourl := me.JsonFile.RepositoryURLParsed(); repourl != nil && len(repourl.Path) > 0 {
			if i := strings.LastIndex(repourl.Path, "."); i > 0 {
				me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, repourl.Path[:i])
			} else {
				me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, repourl.Path)
			}
		}
		if me.GoOut.PkgDirPath = strings.Trim(me.GoOut.PkgDirPath, "/\\"); !strings.HasSuffix(me.GoOut.PkgDirPath, me.JsonFile.Name) {
			me.GoOut.PkgDirPath = filepath.Join(me.GoOut.PkgDirPath, me.JsonFile.Name)
		}
		if len(me.JsonFile.Version) > 0 {
			me.GoOut.PkgDirPath = filepath.Join(me.GoOut.PkgDirPath, me.JsonFile.Version)
		}
		gopkgdir := filepath.Join(Flag.GoDirSrcPath, me.GoOut.PkgDirPath)
		ufs.WalkAllFiles(me.SrcDirPath, func(relpath string) bool {
			if relpath = strings.TrimLeft(relpath[len(me.SrcDirPath):], "\\/"); strings.HasSuffix(relpath, ".purs") {
				me.addModPkgFromPsSrcFileIfCoreimp(relpath, gopkgdir)
			}
			return true
		})
	}
	if err != nil {
		err = errors.New(me.JsonFilePath + ": " + err.Error())
	}
	return
}

func (me *psBowerProject) addModPkgFromPsSrcFileIfCoreimp(relpath string, gopkgdir string) {
	i, l := strings.LastIndexAny(relpath, "/\\"), len(relpath)-5
	modinfo := &modPkg{
		proj: me, srcFilePath: filepath.Join(me.SrcDirPath, relpath),
		qName: strReplSlash2Dot.Replace(relpath[:l]), lName: relpath[i+1 : l],
	}
	if modinfo.impFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "coreimp.json"); ufs.FileExists(modinfo.impFilePath) {
		modinfo.pName = strReplDot2Underscore.Replace(modinfo.qName)
		modinfo.extFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "externs.json")
		modinfo.girMetaFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "gonad.json")
		modinfo.goOutDirPath = relpath[:l]
		modinfo.goOutFilePath = filepath.Join(modinfo.goOutDirPath, modinfo.lName) + ".go"
		modinfo.gopkgfilepath = filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if ufs.FileExists(modinfo.girMetaFilePath) && ufs.FileExists(modinfo.gopkgfilepath) {
			modinfo.reGenGIr = ufs.IsAnyInNewerThanAnyOf(filepath.Dir(modinfo.impFilePath),
				modinfo.girMetaFilePath, modinfo.gopkgfilepath)
		} else {
			modinfo.reGenGIr = true
		}
		me.Modules = append(me.Modules, modinfo)
	}
}

func (me *psBowerProject) forAll(op func(*sync.WaitGroup, *modPkg)) {
	var wg sync.WaitGroup
	for _, modinfo := range me.Modules {
		wg.Add(1)
		go op(&wg, modinfo)
	}
	wg.Wait()
}

func (me *psBowerProject) ensureModPkgGIrMetas() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		var err error
		if modinfo.reGenGIr || Flag.ForceRegenAll {
			err = modinfo.reGenPkgGIrMeta()
		} else if err = modinfo.loadPkgGIrMeta(); err != nil {
			modinfo.reGenGIr = true // we capture this so the .go file later also gets re-gen'd from the re-gen'd girs
			println(modinfo.qName + ": regenerating due to error when loading " + modinfo.girMetaFilePath + ": " + err.Error())
			err = modinfo.reGenPkgGIrMeta()
		}
		if err != nil {
			panic(err)
		}
	})
}

func (me *psBowerProject) prepModPkgGIrAsts() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		if modinfo.reGenGIr || Flag.ForceRegenAll {
			if err := modinfo.prepGIrAst(); err != nil {
				panic(err)
			}
		}
	})
}

func (me *psBowerProject) reGenModPkgGIrAsts() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		if modinfo.reGenGIr || Flag.ForceRegenAll {
			if err := modinfo.reGenPkgGIrAst(); err != nil {
				panic(err)
			}
		}
	})
}

func (me *psBowerProject) writeOutDirtyGIrMetas(isagain bool) (err error) {
	isfirst := !isagain
	me.forAll(func(wg *sync.WaitGroup, m *modPkg) {
		defer wg.Done()
		shouldwrite := (isagain && m.girMeta.save) ||
			(isfirst && (m.reGenGIr || m.girMeta.save || Flag.ForceRegenAll))
		if shouldwrite {
			var buf bytes.Buffer
			if err = m.girMeta.writeAsJsonTo(&buf); err == nil {
				if err = ufs.WriteBinaryFile(m.girMetaFilePath, buf.Bytes()); err == nil {
					m.girMeta.save = false
				}
			}
			if err != nil {
				panic(err)
			}
		}
	})
	return
}
