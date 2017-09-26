package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
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
	GoOut            struct {
		PkgDirPath string
	}
}

func (me *BowerProject) LoadFromJsonFile(isdep bool) (err error) {
	var jsonbytes []byte
	if jsonbytes, err = ioutil.ReadFile(me.JsonFilePath); err == nil {
		if err = json.Unmarshal(jsonbytes, &me.JsonFile); err == nil {
			me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, me.JsonFile.Name)
			if u, _ := url.Parse(me.JsonFile.Repository.URL); u != nil && len(u.Path) > 0 { // yeap, double-check apparently needed ..
				if i := strings.LastIndex(u.Path, "."); i > 0 {
					me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, u.Path[:i])
					if !strings.HasSuffix(me.GoOut.PkgDirPath, me.JsonFile.Name) {
						me.GoOut.PkgDirPath = filepath.Join(me.GoOut.PkgDirPath, me.JsonFile.Name)
					}
				} else if i := strings.LastIndexAny(u.Path, "/\\"); i > 0 {
					me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, u.Path[:i], me.JsonFile.Name)
				} else {
					me.GoOut.PkgDirPath = filepath.Join(Flag.GoNamespace, u.Path)
				}
			}
			println(me.GoOut.PkgDirPath)
		}
	}
	if err != nil {
		err = errors.New(me.JsonFilePath + ": " + err.Error())
	}
	return
}
