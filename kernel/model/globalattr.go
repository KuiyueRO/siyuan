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

	// 如果是自定义块属性，先检查是否已存在同名的
	if req.IsCustomAttr && "" == req.GaID {
		existing, err := FindCustomAttrGAByName(req.Name)
		if nil != err {
			return nil, err
		}
		if nil != existing {
			// 已存在同名的自定义块属性全局属性，直接返回
			return existing, nil
		}
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

	// Validate isCustomAttr constraints
	if req.IsCustomAttr {
		attrName := keyValues.Key.Name
		if "" != req.GaID {
			// If binding to existing GA, get its name
			existingAttr, _ := av.ParseGlobalAttribute(req.GaID)
			if existingAttr != nil && existingAttr.Key != nil {
				attrName = existingAttr.Key.Name
			}
		}
		// Check name validity
		if !IsValidCustomAttrName(attrName) {
			return nil, fmt.Errorf("invalid custom attribute name: %s (must start with a letter and contain only letters, numbers, and hyphens)", attrName)
		}
		// Check for name conflict
		excludeID := req.GaID
		if excludeID == "" {
			excludeID = keyValues.Key.GaID
		}
		conflictID, checkErr := CheckCustomAttrNameConflict(attrName, excludeID)
		if checkErr != nil {
			logging.LogWarnf("check custom attr name conflict failed: %s", checkErr)
		} else if conflictID != "" {
			return nil, fmt.Errorf("another global attribute with name '%s' already has isCustomAttr enabled (gaId: %s)", attrName, conflictID)
		}
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

// IsValidCustomAttrName checks if the name is valid for custom attribute usage.
// Valid names must start with an ASCII letter and contain only ASCII letters, numbers, and hyphens.
func IsValidCustomAttrName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if i == 0 {
			// First character must be a letter
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return false
			}
		} else {
			// Subsequent characters can be letters, numbers, or hyphens
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}

// FindCustomAttrGAByName finds a global attribute with the given name that has isCustomAttr=true.
// Returns nil if no such attribute exists.
func FindCustomAttrGAByName(name string) (*GlobalAttr, error) {
	attrs, err := av.ListGlobalAttributes()
	if err != nil {
		return nil, err
	}
	for _, attr := range attrs {
		if attr != nil && attr.Key != nil && attr.Key.IsCustomAttr && attr.Key.Name == name {
			return convertAttribute(attr), nil
		}
	}
	return nil, nil
}

// CheckCustomAttrNameConflict checks if there's another GA with the same name that already has isCustomAttr=true.
// Returns the conflicting GA ID if found, empty string otherwise.
func CheckCustomAttrNameConflict(name string, excludeGaID string) (string, error) {
	attrs, err := av.ListGlobalAttributes()
	if err != nil {
		return "", err
	}
	for _, attr := range attrs {
		if attr == nil || attr.Key == nil {
			continue
		}
		if attr.Key.IsCustomAttr && attr.Key.Name == name && attr.ID() != excludeGaID {
			return attr.ID(), nil
		}
	}
	return "", nil
}

// BindBlockToGlobalAttr binds a block to a global attribute.
// This updates the block's custom-gas attribute and adds a value entry in the GA.
func BindBlockToGlobalAttr(blockID, gaID string, initialValue interface{}) error {
	if blockID == "" || gaID == "" {
		return errors.New("blockID and gaID are required")
	}

	// Parse the global attribute
	attr, err := av.ParseGlobalAttribute(gaID)
	if err != nil {
		return fmt.Errorf("failed to parse global attribute [%s]: %w", gaID, err)
	}
	if attr == nil || attr.Key == nil {
		return fmt.Errorf("global attribute [%s] not found", gaID)
	}

	// Create initial value
	val := &av.Value{
		ID:         ast.NewNodeID(),
		BlockRefID: blockID,
		Type:       attr.Key.Type,
	}

	// If initial value is provided, try to set it
	if initialValue != nil {
		if data, err := gulu.JSON.MarshalJSON(initialValue); err == nil {
			gulu.JSON.UnmarshalJSON(data, val)
		}
	}

	// Initialize value based on type if not set
	initValueByType(val, attr.Key.Type)

	// Upsert value in GA
	attr.UpsertValue(blockID, val)

	// Save GA
	if err := av.SaveGlobalAttribute(attr); err != nil {
		return fmt.Errorf("failed to save global attribute: %w", err)
	}

	// Update block's custom-gas attribute
	updateBlockCustomGas(nil, blockID, gaID, true)

	// If isCustomAttr, also sync to block's custom attribute
	if attr.Key.IsCustomAttr {
		attrName := "custom-" + attr.Key.Name
		serialized := serializeGAValueForCustomAttr(attr.Key, val)
		if serialized != "" {
			if err := writeBlockAttrs(nil, blockID, map[string]string{attrName: serialized}); err != nil {
				logging.LogWarnf("sync custom attr [%s] to block [%s] failed: %s", attrName, blockID, err)
			}
		}
	}

	return nil
}

// UnbindBlockFromGlobalAttr unbinds a block from a global attribute.
func UnbindBlockFromGlobalAttr(blockID, gaID string) error {
	if blockID == "" || gaID == "" {
		return errors.New("blockID and gaID are required")
	}

	// Parse the global attribute
	attr, err := av.ParseGlobalAttribute(gaID)
	if err != nil {
		return fmt.Errorf("failed to parse global attribute [%s]: %w", gaID, err)
	}
	if attr == nil {
		// GA doesn't exist, just update the block
		updateBlockCustomGas(nil, blockID, gaID, false)
		return nil
	}

	// Remove value from GA
	attr.RemoveValue(blockID)

	// Save GA
	if err := av.SaveGlobalAttribute(attr); err != nil {
		return fmt.Errorf("failed to save global attribute: %w", err)
	}

	// Update block's custom-gas attribute
	updateBlockCustomGas(nil, blockID, gaID, false)

	// If isCustomAttr, also clear block's custom attribute
	if attr.Key != nil && attr.Key.IsCustomAttr {
		attrName := "custom-" + attr.Key.Name
		if err := writeBlockAttrs(nil, blockID, map[string]string{attrName: ""}); err != nil {
			logging.LogWarnf("clear custom attr [%s] from block [%s] failed: %s", attrName, blockID, err)
		}
	}

	return nil
}

// initValueByType initializes a value based on its type
func initValueByType(val *av.Value, keyType av.KeyType) {
	if val == nil {
		return
	}
	switch keyType {
	case av.KeyTypeText:
		if val.Text == nil {
			val.Text = &av.ValueText{}
		}
	case av.KeyTypeNumber:
		if val.Number == nil {
			val.Number = &av.ValueNumber{}
		}
	case av.KeyTypeDate:
		if val.Date == nil {
			val.Date = &av.ValueDate{}
		}
	case av.KeyTypeSelect, av.KeyTypeMSelect:
		if val.MSelect == nil {
			val.MSelect = []*av.ValueSelect{}
		}
	case av.KeyTypeURL:
		if val.URL == nil {
			val.URL = &av.ValueURL{}
		}
	case av.KeyTypeEmail:
		if val.Email == nil {
			val.Email = &av.ValueEmail{}
		}
	case av.KeyTypePhone:
		if val.Phone == nil {
			val.Phone = &av.ValuePhone{}
		}
	case av.KeyTypeMAsset:
		if val.MAsset == nil {
			val.MAsset = []*av.ValueAsset{}
		}
	case av.KeyTypeCheckbox:
		if val.Checkbox == nil {
			val.Checkbox = &av.ValueCheckbox{}
		}
	case av.KeyTypeRelation:
		if val.Relation == nil {
			val.Relation = &av.ValueRelation{}
		}
	case av.KeyTypeRollup:
		if val.Rollup == nil {
			val.Rollup = &av.ValueRollup{}
		}
	}
}
