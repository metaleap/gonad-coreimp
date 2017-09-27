package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
			if err = ufs.EnsureDirExists(gopkgdir); err == nil {
				repl := strings.NewReplacer("\\", ".", "/", ".")
				ufs.WalkAllFiles(me.SrcDirPath, func(relpath string) bool {
					if relpath = strings.TrimLeft(relpath[len(me.SrcDirPath):], "\\/"); strings.HasSuffix(relpath, ".purs") {
						i, l := strings.LastIndexAny(relpath, "/\\"), len(relpath)-5
						modinfo := &ModuleInfo{srcFilePath: filepath.Join(me.SrcDirPath, relpath), qName: repl.Replace(relpath[:l])}
						modinfo.impFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "coreimp.json")
						modinfo.extFilePath = filepath.Join(Proj.DumpsDirProjPath, modinfo.qName, "externs.json")
						modinfo.goOutFilePath = filepath.Join(relpath[:l], relpath[i+1:l]) + ".go"
						gopkgfile := filepath.Join(gopkgdir, modinfo.goOutFilePath)
						if !ufs.FileExists(gopkgfile) {
							modinfo.regenerate = true
						} else if !ufs.FileExists(modinfo.impFilePath) {
							modinfo.regenerate = false
							os.RemoveAll(gopkgdir)
						} else {
							modinfo.regenerate, _ = ufs.IsNewerThan(modinfo.impFilePath, gopkgfile)
						}
						me.Modules = append(me.Modules, modinfo)
					}
					return true
				})
			}
		}
	}
	if err != nil {
		err = errors.New(me.JsonFilePath + ": " + err.Error())
	}
	return
}
