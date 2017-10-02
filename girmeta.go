package main

import (
	"encoding/json"
	"errors"
	"io"
	"path"
)

type GonadIrMeta struct {
	Imports     []GIrMPkgRef
	TypeAliases []GIrMTypeAlias

	GoTypeDefs []*GIrATypeDef `json:",omitempty"`

	imports []*ModuleInfo

	mod  *ModuleInfo
	proj *BowerProject
	save bool
}

type GIrMPkgRef struct {
	N string
	Q string
	P string

	used bool
}

func (me *GonadIrMeta) PopulateFromCoreImp() (err error) {
	for _, impname := range me.mod.coreimp.Imports {
		if impname != "Prim" && impname != "Prelude" && impname != me.mod.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}
	me.populateTypeAliases()
	me.populateGoTypeDefs()

	if err == nil {
		for _, impmod := range me.imports {
			me.Imports = append(me.Imports, GIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)})
		}
	}
	return
}

func (me *GonadIrMeta) PopulateFromLoaded() error {
	me.imports = nil
	for _, imp := range me.Imports {
		if impmod := FindModuleByQName(imp.Q); impmod == nil {
			return errors.New("Bad import " + imp.Q)
		} else {
			me.imports = append(me.imports, impmod)
		}
	}
	return nil
}

func (me *GonadIrMeta) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
