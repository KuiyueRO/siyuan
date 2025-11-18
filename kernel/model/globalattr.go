// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package model

import (
	"errors"
	"fmt"

	"github.com/88250/gulu"
	"github.com/88250/lute/ast"
	"github.com/siyuan-note/logging"
	"github.com/siyuan-note/siyuan/kernel/av"
)

// GlobalAttr 描述了后端向前端返回的全局属性元信息。
type GlobalAttr struct {
	GaID         string             `json:"gaId"`
	Name         string             `json:"name"`
	Icon         string             `json:"icon"`
	Desc         string             `json:"desc"`
	Type         av.KeyType         `json:"type"`
	Options      []*av.SelectOption `json:"options,omitempty"`
	NumberFormat av.NumberFormat    `json:"numberFormat"`
	Template     string             `json:"template"`
	IsCustomAttr bool               `json:"isCustomAttr"`
}

// CreateGlobalAttrReq 为 /api/globalattr/create 提供参数。
type CreateGlobalAttrReq struct {
	GaID         string             `json:"gaId"`
	Name         string             `json:"name"`
	Icon         string             `json:"icon"`
	Desc         string             `json:"desc"`
	Type         string             `json:"type"`
	Options      []*av.SelectOption `json:"options"`
	NumberFormat av.NumberFormat    `json:"numberFormat"`
	Template     string             `json:"template"`
	IsCustomAttr bool               `json:"isCustomAttr"`
}

// MarkGlobalAttrReq 表示 AV 列与全局属性之间的绑定请求。
type MarkGlobalAttrReq struct {
	AvID           string `json:"avID"`
	KeyID          string `json:"keyID"`
	GaID           string `json:"gaId"`
	IsCustomAttr   bool   `json:"isCustomAttr"`
	CreateIfAbsent bool   `json:"createIfAbsent"`
	Enabled        bool   `json:"enabled"`
}

// ListGlobalAttrs 返回全部全局属性的概览。
func ListGlobalAttrs() ([]*GlobalAttr, error) {
	attrs, err := av.ListGlobalAttributes()
	if nil != err {
		return nil, err
	}

	seen := map[string]bool{}
	ret := append([]*GlobalAttr{}, builtinGlobalAttrs()...)
	for _, attr := range ret {
		seen[attr.GaID] = true
	}

	for _, attr := range attrs {
		if ga := convertAttribute(attr); nil != ga {
			if seen[ga.GaID] {
				continue
			}
			seen[ga.GaID] = true
			ret = append(ret, ga)
		}
	}
	return ret, nil
}

// CreateGlobalAttr 写入新的全局属性。
func CreateGlobalAttr(req *CreateGlobalAttrReq) (*GlobalAttr, error) {
	if nil == req {
		return nil, errors.New("empty request")
	}

	keyType, err := normalizeKeyType(req.Type)
	if nil != err {
		return nil, err
	}

	gaID := req.GaID
	if "" == gaID {
		gaID = ast.NewNodeID()
	}

	key := &av.Key{
		ID:           gaID,
		GaID:         gaID,
		Name:         req.Name,
		Icon:         req.Icon,
		Desc:         req.Desc,
		Type:         keyType,
		Options:      req.Options,
		NumberFormat: req.NumberFormat,
		Template:     req.Template,
		IsCustomAttr: req.IsCustomAttr,
	}

	attr := &av.GlobalAttribute{Key: key}
	if err = av.SaveGlobalAttribute(attr); nil != err {
		return nil, err
	}
	return convertAttribute(attr), nil
}

// MarkAttrViewColumnAsGlobal 将 AV 列与指定/新建的全局属性绑定。
func MarkAttrViewColumnAsGlobal(req *MarkGlobalAttrReq) (*GlobalAttr, error) {
	if nil == req {
		return nil, errors.New("empty request")
	}
	if "" == req.AvID || "" == req.KeyID {
		return nil, errors.New("avID or keyID is empty")
	}

	attrView, err := av.ParseAttributeView(req.AvID)
	if nil != err {
		return nil, err
	}

	keyValues, err := attrView.GetKeyValues(req.KeyID)
	if nil != err {
		return nil, err
	}

	if !req.Enabled {
		keyValues.Key.GaID = ""
		keyValues.Key.IsCustomAttr = false
		if err = av.SaveAttributeView(attrView); nil != err {
			return nil, err
		}
		return nil, nil
	}

	if builtin := builtinGlobalAttrByID(req.GaID); nil != builtin {
		keyValues.Key.GaID = builtin.GaID
		keyValues.Key.IsCustomAttr = false
		if err = av.SaveAttributeView(attrView); nil != err {
			return nil, err
		}
		return builtin, nil
	}

	var attr *av.GlobalAttribute
	if "" != req.GaID {
		attr, err = av.ParseGlobalAttribute(req.GaID)
		if nil != err && !req.CreateIfAbsent {
			return nil, err
		}
	}

	if nil == attr {
		attr, err = cloneAttrFromKey(keyValues.Key, req.IsCustomAttr)
		if nil != err {
			return nil, err
		}
		if "" != req.GaID {
			attr.Key.ID = req.GaID
			attr.Key.GaID = req.GaID
		}
		if err = av.SaveGlobalAttribute(attr); nil != err {
			return nil, err
		}
	}

	if attr.Key.IsCustomAttr != req.IsCustomAttr {
		attr.Key.IsCustomAttr = req.IsCustomAttr
		if err = av.SaveGlobalAttribute(attr); nil != err {
			logging.LogWarnf("update global attribute [%s] custom flag failed: %s", attr.ID(), err)
		}
	}

	keyValues.Key.GaID = attr.ID()
	keyValues.Key.IsCustomAttr = req.IsCustomAttr
	if err = ensureGlobalAttrColumnBindings(attrView, keyValues, attr); nil != err {
		return nil, err
	}

	if err = av.SaveAttributeView(attrView); nil != err {
		return nil, err
	}
	return convertAttribute(attr), nil
}

func convertAttribute(attr *av.GlobalAttribute) *GlobalAttr {
	if nil == attr || nil == attr.Key {
		return nil
	}
	return &GlobalAttr{
		GaID:         attr.ID(),
		Name:         attr.Key.Name,
		Icon:         attr.Key.Icon,
		Desc:         attr.Key.Desc,
		Type:         attr.Key.Type,
		Options:      attr.Key.Options,
		NumberFormat: attr.Key.NumberFormat,
		Template:     attr.Key.Template,
		IsCustomAttr: attr.Key.IsCustomAttr,
	}
}

func cloneAttrFromKey(source *av.Key, isCustom bool) (*av.GlobalAttribute, error) {
	if nil == source {
		return nil, errors.New("nil key")
	}

	cloned := &av.Key{}
	data, err := gulu.JSON.MarshalJSON(source)
	if nil != err {
		return nil, err
	}
	if err = gulu.JSON.UnmarshalJSON(data, cloned); nil != err {
		return nil, err
	}

	gaID := ast.NewNodeID()
	cloned.ID = gaID
	cloned.GaID = gaID
	cloned.IsCustomAttr = isCustom

	return &av.GlobalAttribute{Key: cloned}, nil
}

func ensureGlobalAttrColumnBindings(attrView *av.AttributeView, keyValues *av.KeyValues, attr *av.GlobalAttribute) error {
	if attrView == nil || keyValues == nil || attr == nil {
		return nil
	}

	blockMap := map[string]*av.Value{}
	if blockKeyValues := attrView.GetBlockKeyValues(); blockKeyValues != nil {
		for _, blockVal := range blockKeyValues.Values {
			blockMap[blockVal.BlockID] = blockVal
		}
	}

	var changed bool
	for _, val := range keyValues.Values {
		if val == nil {
			continue
		}
		blockVal := blockMap[val.BlockID]
		blockID := getBoundBlockID(blockVal)
		if blockID == "" {
			val.BlockRefID = ""
			continue
		}
		if val.BlockRefID != blockID {
			val.BlockRefID = blockID
		}
		attr.UpsertValue(blockID, val)
		changed = true
	}

	if !changed {
		return nil
	}
	return av.SaveGlobalAttribute(attr)
}

func getBoundBlockID(blockVal *av.Value) string {
	if blockVal == nil || blockVal.IsDetached || blockVal.Block == nil {
		return ""
	}
	return blockVal.Block.ID
}

func normalizeKeyType(raw string) (av.KeyType, error) {
	if "" == raw {
		return av.KeyTypeText, nil
	}
	typ := av.KeyType(raw)
	switch typ {
	case av.KeyTypeBlock,
		av.KeyTypeText,
		av.KeyTypeNumber,
		av.KeyTypeDate,
		av.KeyTypeSelect,
		av.KeyTypeMSelect,
		av.KeyTypeURL,
		av.KeyTypeEmail,
		av.KeyTypePhone,
		av.KeyTypeMAsset,
		av.KeyTypeTemplate,
		av.KeyTypeCreated,
		av.KeyTypeUpdated,
		av.KeyTypeCheckbox,
		av.KeyTypeRelation,
		av.KeyTypeRollup,
		av.KeyTypeLineNumber:
		return typ, nil
	}
	return "", fmt.Errorf("unsupported key type: %s", raw)
}

func builtinGlobalAttrs() []*GlobalAttr {
	return collectBuiltinGlobalAttrs()
}

func builtinGlobalAttrByID(id string) *GlobalAttr {
	if "" == id {
		return nil
	}
	if spec := builtinAttrSpecMap[id]; spec != nil {
		return spec.attr
	}
	return nil
}

func newBuiltinGlobalAttr(id, name string, keyType av.KeyType, desc string) *GlobalAttr {
	return &GlobalAttr{
		GaID:         id,
		Name:         name,
		Desc:         desc,
		Type:         keyType,
		IsCustomAttr: false,
	}
}
