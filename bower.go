package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/metaleap/go-util-fs"
)

type BowerFile struct {
	Name        string `json:"name"`
	HomePage    string `json:"homepage,omitempty"`
	Description string `json:"description,omitempty"`
	License     string `json:"license,omitempty"`

	Repository struct {
		Type string `json:"type,omitempty"`
		URL  string `json:"url,omitempty"`
	} `json:"repository,omitempty"`
	Ignore            []string          `json:"ignore,omitempty"`
	Dependencies      map[string]string `json:"dependencies,omitempty"`
	DevDependencies   map[string]string `json:"devDependencies,omitempty"`
	GonadDependencies map[string]string `json:"gonadDependencies,omitempty"`

	Version     string `json:"version,omitempty"`
	_Release    string `json:"_release,omitempty"`
	_Resolution struct {
		Type   string `json:"type,omitempty"`
		Tag    string `json:"tag,omitempty"`
		Commit string `json:"commit,omitempty"`
	} `json:"_resolution,omitempty"`
	_Source         string `json:"_source,omitempty"`
	_Target         string `json:"_target,omitempty"`
	_OriginalSource string `json:"_originalSource,omitempty"`
	_Direct         bool   `json:"_direct,omitempty"`
}

type BowerProject struct {
	JsonFilePath     string
	SrcDirPath       string
	DepsDirPath      string
	DumpsDirProjPath string
	JsonFile         BowerFile
	Modules          []*ModuleInfo
	GoOut            struct {
		PkgDirPath string
	}
}

func (me *BowerProject) EnsureOutDirs() (err error) {
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

func (me *BowerProject) ModuleByQName(qname string) *ModuleInfo {
	for _, m := range me.Modules {
		if m.qName == qname {
			return m
		}
	}
	return nil
}

func (me *BowerProject) LoadFromJsonFile(isdep bool) (err error) {
	var jsonbytes []byte
	if jsonbytes, err = ioutil.ReadFile(me.JsonFilePath); err == nil {
		if err = json.Unmarshal(jsonbytes, &me.JsonFile); err == nil {
			me.GoOut.PkgDirPath = Flag.GoNamespace
			if u, _ := url.Parse(me.JsonFile.Repository.URL); u != nil && len(u.Path) > 0 { // yeap, double-check apparently needed ..
				if i := strings.LastIndex(u.Path, "."); i > 0 {
					me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, u.Path[:i])
				} else {
					me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, u.Path)
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
					me.AddModuleInfoFromPsSrcFileIfCoreimp(relpath, gopkgdir)
				}
				return true
			})
		}
	}
	if err != nil {
		err = errors.New(me.JsonFilePath + ": " + err.Error())
	}
	return
}

func (me *BowerProject) AddModuleInfoFromPsSrcFileIfCoreimp(relpath string, gopkgdir string) {
	i, l := strings.LastIndexAny(relpath, "/\\"), len(relpath)-5
	modinfo := &ModuleInfo{
		proj: me, srcFilePath: filepath.Join(me.SrcDirPath, relpath),
		qName: slash2dot.Replace(relpath[:l]), lName: relpath[i+1 : l],
	}
	if modinfo.impFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "coreimp.json"); ufs.FileExists(modinfo.impFilePath) {
		modinfo.pName = dot2underscore.Replace(modinfo.qName)
		modinfo.extFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "externs.json")
		modinfo.girMetaFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "gonadmeta.json")
		modinfo.girAstFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "gonadast.json")
		modinfo.goOutDirPath = relpath[:l]
		modinfo.goOutFilePath = filepath.Join(modinfo.goOutDirPath, modinfo.lName) + ".go"
		modinfo.gopkgfilepath = filepath.Join(gopkgdir, modinfo.goOutFilePath)
		if ufs.FileExists(modinfo.girMetaFilePath) && ufs.FileExists(modinfo.girAstFilePath) && ufs.FileExists(modinfo.gopkgfilepath) {
			modinfo.reGenGIr = ufs.IsAnyInNewerThanAnyOf(filepath.Dir(modinfo.impFilePath),
				modinfo.girMetaFilePath, modinfo.girAstFilePath, modinfo.gopkgfilepath)
		} else {
			modinfo.reGenGIr = true
		}
		me.Modules = append(me.Modules, modinfo)
	}
}

func (me *BowerProject) EnsureModPkgGIrMetas() {
	var wg sync.WaitGroup
	ensuregirmeta := func(modinfo *ModuleInfo) {
		var err error
		defer wg.Done()
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
	}
	for _, modinfo := range me.Modules {
		wg.Add(1)
		go ensuregirmeta(modinfo)
	}
	wg.Wait()
}

func (me *BowerProject) PrepOrReGenModPkgGIrAsts(prep bool) {
	var wg sync.WaitGroup
	regenorprep := func(modinfo *ModuleInfo) {
		defer wg.Done()
		fn := modinfo.prepGIrAst
		if !prep {
			fn = modinfo.reGenPkgGIrAst
		}
		if err := fn(); err != nil {
			panic(err)
		}
	}
	for _, modinfo := range me.Modules {
		if modinfo.reGenGIr || Flag.ForceRegenAll {
			wg.Add(1)
			go regenorprep(modinfo)
		}
	}
	wg.Wait()
}

func (me *BowerProject) WriteOutDirtyGIrMetas(isagain bool) (err error) {
	var buf bytes.Buffer
	for _, m := range me.Modules {
		isfirst := !isagain
		dosave := (isagain && m.girMeta.save) ||
			(isfirst && (m.reGenGIr || Flag.ForceRegenAll || m.girMeta.save))
		if dosave {
			if err = m.girMeta.WriteAsJsonTo(&buf); err == nil {
				if err = ufs.WriteBinaryFile(m.girMetaFilePath, buf.Bytes()); err == nil {
					m.girMeta.save = false
				}
				buf.Reset()
			}
		}
		if err != nil {
			return
		}
	}
	return
}
