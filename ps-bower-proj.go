package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util/dev/bower"
	"github.com/metaleap/go-util/dev/go"
	"github.com/metaleap/go-util/fs"
)

/*
Represents either the given PureScript main `src` project
or one of its dependency libs usually found in `bower_components`.
*/

type psBowerFile struct {
	udevbower.BowerFile

	Gonad struct { // all settings in here apply to all Deps equally as they do to the main Proj --- ie. the former get a copy of the latter, ignoring their own Gonad field even if present
		In struct {
			CoreImpDumpsDirPath string // dir path containing Some.Module.QName/coreimp.json files
		}
		Out struct {
			ForceAll     bool   // if false, only regenerate packages that are out of date with respect to coreimp.json or externs.json
			DumpAst      bool   // dumps an additional gonad.ast.json next to gonad.json
			MainDepLevel int    // temporary option
			GoDirSrcPath string // defaults to the first `GOPATH` found that has a `src` sub-directory
			GoNamespace  string // defaults to github.com/gonadz (or github.com\gonadz under Windows). only used to construct psBowerProject.GoOut.PkgDirPath
		}
		CodeGen struct {
			// TypeClasses2Interfaces bool
			// SaturateFuncArities    bool
			FlattenIfs             bool
			PtrStructMinFieldCount int
		}

		loadedFromJson bool
	}
}

type psBowerProject struct {
	BowerJsonFile     psBowerFile
	BowerJsonFilePath string
	DepsDirPath       string
	SrcDirPath        string
	Modules           []*modPkg
	GoOut             struct {
		PkgDirPath string
	}
}

func (me *psBowerProject) ensureOutDirs() (err error) {
	dirpath := filepath.Join(Proj.BowerJsonFile.Gonad.Out.GoDirSrcPath, me.GoOut.PkgDirPath)
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
	if qname != "" {
		for _, m := range me.Modules {
			if m.qName == qname {
				return m
			}
		}
	}
	return nil
}

func (me *psBowerProject) moduleByPName(pname string) *modPkg {
	if pname != "" {
		for _, m := range me.Modules {
			if m.pName == pname {
				return m
			}
		}
	}
	return nil
}

func (me *psBowerProject) loadFromJsonFile() (err error) {
	if err = udevbower.LoadFromFile(me.BowerJsonFilePath, &me.BowerJsonFile); err == nil {
		// populate defaults for Gonad sub-fields
		cfg, isdep := &me.BowerJsonFile.Gonad, me != &Proj
		if isdep {
			cfg = &Proj.BowerJsonFile.Gonad
		} else {
			if cfg.In.CoreImpDumpsDirPath == "" {
				cfg.In.CoreImpDumpsDirPath = "output"
			}
			if cfg.Out.GoNamespace == "" {
				cfg.Out.GoNamespace = filepath.Join("github.com", "gonadz")
			}
			if cfg.Out.GoDirSrcPath == "" {
				for _, gopath := range udevgo.AllGoPaths() {
					if cfg.Out.GoDirSrcPath = filepath.Join(gopath, "src"); ufs.DirExists(cfg.Out.GoDirSrcPath) {
						break
					}
				}
			}
			if cfg.CodeGen.PtrStructMinFieldCount == 0 {
				cfg.CodeGen.PtrStructMinFieldCount = 2
			}
			err = ufs.EnsureDirExists(cfg.Out.GoDirSrcPath)
			cfg.loadedFromJson = true
		}
		if err == nil {
			// proceed
			me.GoOut.PkgDirPath = cfg.Out.GoNamespace
			if repourl := me.BowerJsonFile.RepositoryURLParsed(); repourl != nil && repourl.Path != "" {
				if i := strings.LastIndex(repourl.Path, "."); i > 0 {
					me.GoOut.PkgDirPath = filepath.Join(cfg.Out.GoNamespace, repourl.Path[:i])
				} else {
					me.GoOut.PkgDirPath = filepath.Join(cfg.Out.GoNamespace, repourl.Path)
				}
			}
			if me.GoOut.PkgDirPath = strings.Trim(me.GoOut.PkgDirPath, "/\\"); !strings.HasSuffix(me.GoOut.PkgDirPath, me.BowerJsonFile.Name) {
				me.GoOut.PkgDirPath = filepath.Join(me.GoOut.PkgDirPath, me.BowerJsonFile.Name)
			}
			if me.BowerJsonFile.Version != "" {
				me.GoOut.PkgDirPath = filepath.Join(me.GoOut.PkgDirPath, me.BowerJsonFile.Version)
			}
			gopkgdir := filepath.Join(cfg.Out.GoDirSrcPath, me.GoOut.PkgDirPath)
			ufs.WalkAllFiles(me.SrcDirPath, func(relpath string) bool {
				if relpath = strings.TrimLeft(relpath[len(me.SrcDirPath):], "\\/"); strings.HasSuffix(relpath, ".purs") {
					me.addModPkgFromPsSrcFileIfCoreimp(relpath, gopkgdir)
				}
				return true
			})
		}
	}
	if err != nil {
		err = errors.New(me.BowerJsonFilePath + ": " + err.Error())
	}
	return
}

func (me *psBowerProject) addModPkgFromPsSrcFileIfCoreimp(relpath string, gopkgdir string) {
	i, l, opt := strings.LastIndexAny(relpath, "/\\"), len(relpath)-5, Proj.BowerJsonFile.Gonad
	modinfo := &modPkg{
		proj: me, srcFilePath: filepath.Join(me.SrcDirPath, relpath),
		qName: strReplSlash2Dot.Replace(relpath[:l]), lName: relpath[i+1 : l],
	}
	if modinfo.impFilePath = filepath.Join(opt.In.CoreImpDumpsDirPath, modinfo.qName, "coreimp.json"); ufs.FileExists(modinfo.impFilePath) {
		modinfo.pName = strReplDot2Underscore.Replace(modinfo.qName)
		modinfo.extFilePath = filepath.Join(opt.In.CoreImpDumpsDirPath, modinfo.qName, "externs.json")
		modinfo.irMetaFilePath = filepath.Join(opt.In.CoreImpDumpsDirPath, modinfo.qName, "gonad.json")
		modinfo.goOutDirPath = relpath[:l]
		modinfo.goOutFilePath = filepath.Join(modinfo.goOutDirPath, modinfo.qName) + ".go"
		modinfo.gopkgfilepath = filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if ufs.FileExists(modinfo.irMetaFilePath) && ufs.FileExists(modinfo.gopkgfilepath) {
			stalemetaˇimp, _ := ufs.IsNewerThan(modinfo.impFilePath, modinfo.irMetaFilePath)
			stalepkgˇimp, _ := ufs.IsNewerThan(modinfo.impFilePath, modinfo.gopkgfilepath)
			stalemetaˇext, _ := ufs.IsNewerThan(modinfo.extFilePath, modinfo.irMetaFilePath)
			stalepkgˇext, _ := ufs.IsNewerThan(modinfo.extFilePath, modinfo.gopkgfilepath)
			modinfo.reGenIr = stalemetaˇimp || stalepkgˇimp || stalemetaˇext || stalepkgˇext
		} else {
			modinfo.reGenIr = true
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

func (me *psBowerProject) ensureModPkgIrMetas() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		var err error
		if modinfo.reGenIr || Proj.BowerJsonFile.Gonad.Out.ForceAll {
			err = modinfo.reGenPkgIrMeta()
		} else if err = modinfo.loadPkgIrMeta(); err != nil {
			modinfo.reGenIr = true // we capture this so the .go file later also gets re-gen'd from the re-gen'd IRs
			println(modinfo.qName + ": regenerating due to error when loading " + modinfo.irMetaFilePath + ": " + err.Error())
			err = modinfo.reGenPkgIrMeta()
		}
		if err != nil {
			panic(err)
		}
	})
}

func (me *psBowerProject) populateModPkgIrMetas() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		modinfo.populatePkgIrMeta()
	})
}

func (me *psBowerProject) prepModPkirAsts() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		if modinfo.reGenIr || Proj.BowerJsonFile.Gonad.Out.ForceAll {
			modinfo.prepIrAst()
		}
	})
}

func (me *psBowerProject) reGenModPkirAsts() {
	me.forAll(func(wg *sync.WaitGroup, modinfo *modPkg) {
		defer wg.Done()
		if modinfo.reGenIr || Proj.BowerJsonFile.Gonad.Out.ForceAll {
			if err := modinfo.reGenPkirAst(); err != nil {
				panic(err)
			}
		}
	})
}

func (me *psBowerProject) writeOutDirtyIrMetas(isagain bool) (err error) {
	isfirst := !isagain
	me.forAll(func(wg *sync.WaitGroup, m *modPkg) {
		defer wg.Done()
		shouldwrite := (isagain && m.irMeta.save) ||
			(isfirst && (m.reGenIr || m.irMeta.save || Proj.BowerJsonFile.Gonad.Out.ForceAll))
		if shouldwrite {
			var buf bytes.Buffer
			if err = m.irMeta.writeAsJsonTo(&buf); err == nil {
				if err = ufs.WriteBinaryFile(m.irMetaFilePath, buf.Bytes()); err == nil {
					m.irMeta.save = false
				}
			}
			if err != nil {
				panic(err)
			}
		}
	})
	return
}
