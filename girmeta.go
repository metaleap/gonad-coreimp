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

	imports []*ModuleInfo

	modinfo *ModuleInfo
	proj    *BowerProject
}

type GIrMPkgRef struct {
	N string
	Q string
	P string
}

type GIrMTypeAlias struct {
	Name      string
	SimpleRef string   `json:",omitempty"`
	MultiRef  []string `json:",omitempty"`
}

func (me *GonadIrMeta) PopulateFromCoreImp() (err error) {
	//	IMPORTS
	for _, impname := range me.modinfo.coreimp.Imports {
		if impname != "Prim" && impname != "Prelude" && impname != me.modinfo.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}

	//	TYPE ALIASES
	for _, d := range me.modinfo.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.modinfo.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMTypeAlias{Name: d.EDTypeSynonym.Name}
			if d.EDTypeSynonym.Type.Tag == "TypeConstructor" {
				for _, s := range d.EDTypeSynonym.Type.Contents.([]interface{})[0].([]interface{}) {
					ta.SimpleRef += (s.(string) + ".")
				}
				ta.SimpleRef += d.EDTypeSynonym.Type.Contents.([]interface{})[1].(string)
			} else if d.EDTypeSynonym.Type.Tag == "TypeApp" {
				//	flatten recursing tree annoyance
				dis, accum := d.EDTypeSynonym.Type, []TaggedContents{}
				var flattenone func(tc map[string]interface{})
				flatten := func() {
					disc := dis.Contents.([]interface{})
					tc1, tc2 := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
					flattenone(tc1)
					flattenone(tc2)
				}
				flattenone = func(tc map[string]interface{}) {
					switch tc["tag"].(string) {
					case "TypeConstructor":
						accum = append(accum, newTaggedContents(tc))
					case "TypeApp":
						olddis, nudis := dis, newTaggedContents(tc)
						dis = &nudis
						flatten()
						dis = olddis
					}
					return
				}
				flatten()
				for _, tc := range accum {
					qname := ""
					for _, s := range tc.Contents.([]interface{})[0].([]interface{}) {
						qname += (s.(string) + ".")
					}
					qname += tc.Contents.([]interface{})[1].(string)
					ta.MultiRef = append(ta.MultiRef, qname)
				}
			} else {
				println(d.EDTypeSynonym.Name + "::" + d.EDTypeSynonym.Type.Tag)
			}
			me.TypeAliases = append(me.TypeAliases, ta)
		}
	}

	//	SET PUBLIC FIELDS USED FOR JSON SERIALIZATION
	if err == nil {
		for _, impmod := range me.imports {
			me.Imports = append(me.Imports, GIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)})
		}
	}
	return
}

func (me *GonadIrMeta) PopulateFromLoaded() (err error) {
	me.imports = nil
	for _, imp := range me.Imports {
		if impmod := FindModuleByQName(imp.Q); impmod == nil {
			err = errors.New("Bad import " + imp.Q)
		} else {
			me.imports = append(me.imports, impmod)
		}
	}

	return
}

func (me *GonadIrMeta) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
