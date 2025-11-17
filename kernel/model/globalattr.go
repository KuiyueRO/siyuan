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
	"github.com/siyuan-note/siyuan/kernel/globalattr"
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
	attrs, err := globalattr.ListAttributes()
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

	attr := &globalattr.Attribute{Key: key}
	if err = globalattr.Save(attr); nil != err {
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

	var attr *globalattr.Attribute
	if "" != req.GaID {
		attr, err = globalattr.Parse(req.GaID)
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
		if err = globalattr.Save(attr); nil != err {
			return nil, err
		}
	}

	if attr.Key.IsCustomAttr != req.IsCustomAttr {
		attr.Key.IsCustomAttr = req.IsCustomAttr
		if err = globalattr.Save(attr); nil != err {
			logging.LogWarnf("update global attribute [%s] custom flag failed: %s", attr.ID(), err)
		}
	}

	keyValues.Key.GaID = attr.ID()
	keyValues.Key.IsCustomAttr = req.IsCustomAttr

	if err = av.SaveAttributeView(attrView); nil != err {
		return nil, err
	}
	return convertAttribute(attr), nil
}

func convertAttribute(attr *globalattr.Attribute) *GlobalAttr {
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

func cloneAttrFromKey(source *av.Key, isCustom bool) (*globalattr.Attribute, error) {
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

	return &globalattr.Attribute{Key: cloned}, nil
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
	return []*GlobalAttr{
		newBuiltinGlobalAttr("id", "ID", av.KeyTypeText, "块 ID"),
		newBuiltinGlobalAttr("parentId", "Parent ID", av.KeyTypeText, "父块 ID"),
		newBuiltinGlobalAttr("rootId", "Root ID", av.KeyTypeText, "根块 ID"),
		newBuiltinGlobalAttr("hash", "Hash", av.KeyTypeText, "内容哈希"),
		newBuiltinGlobalAttr("box", "Box", av.KeyTypeText, "笔记本盒子"),
		newBuiltinGlobalAttr("path", "Path", av.KeyTypeText, "文档路径"),
		newBuiltinGlobalAttr("hPath", "HPath", av.KeyTypeText, "人类可读路径"),
		newBuiltinGlobalAttr("name", "Name", av.KeyTypeText, "名称"),
		newBuiltinGlobalAttr("alias", "Alias", av.KeyTypeText, "别名"),
		newBuiltinGlobalAttr("memo", "Memo", av.KeyTypeText, "备注"),
		newBuiltinGlobalAttr("tag", "Tag", av.KeyTypeText, "标签"),
		newBuiltinGlobalAttr("content", "Content", av.KeyTypeText, "原始内容"),
		newBuiltinGlobalAttr("fcontent", "FContent", av.KeyTypeText, "格式化内容"),
		newBuiltinGlobalAttr("markdown", "Markdown", av.KeyTypeText, "Markdown 内容"),
		newBuiltinGlobalAttr("length", "Length", av.KeyTypeNumber, "内容长度"),
		newBuiltinGlobalAttr("type", "Type", av.KeyTypeText, "块类型"),
		newBuiltinGlobalAttr("subType", "Subtype", av.KeyTypeText, "子类型"),
		newBuiltinGlobalAttr("ial", "IAL", av.KeyTypeText, "属性 IAL"),
		newBuiltinGlobalAttr("sort", "Sort", av.KeyTypeNumber, "排序值"),
		newBuiltinGlobalAttr("created", "Created", av.KeyTypeText, "创建时间"),
		newBuiltinGlobalAttr("updated", "Updated", av.KeyTypeText, "更新时间"),
	}
}

func builtinGlobalAttrByID(id string) *GlobalAttr {
	if "" == id {
		return nil
	}
	for _, attr := range builtinGlobalAttrs() {
		if attr.GaID == id {
			return attr
		}
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
