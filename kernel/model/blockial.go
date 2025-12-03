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
	"strconv"
	"strings"
	"time"

	"github.com/88250/gulu"
	"github.com/88250/lute/ast"
	"github.com/88250/lute/editor"
	"github.com/88250/lute/lex"
	"github.com/88250/lute/parse"
	"github.com/araddon/dateparse"
	"github.com/siyuan-note/logging"
	"github.com/siyuan-note/siyuan/kernel/av"
	"github.com/siyuan-note/siyuan/kernel/cache"
	"github.com/siyuan-note/siyuan/kernel/filesys"
	"github.com/siyuan-note/siyuan/kernel/sql"
	"github.com/siyuan-note/siyuan/kernel/treenode"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func SetBlockReminder(id string, timed string) (err error) {
	if !IsSubscriber() {
		if "ios" == util.Container {
			return errors.New(Conf.Language(122))
		}
		return errors.New(Conf.Language(29))
	}

	var timedMills int64
	if "0" != timed {
		t, e := dateparse.ParseIn(timed, time.Now().Location())
		if nil != e {
			return e
		}
		timedMills = t.UnixMilli()
	}

	FlushTxQueue()

	attrs := sql.GetBlockAttrs(id)
	tree, err := LoadTreeByBlockID(id)
	if err != nil {
		return
	}

	node := treenode.GetNodeInTree(tree, id)
	if nil == node {
		return errors.New(fmt.Sprintf(Conf.Language(15), id))
	}

	if ast.NodeDocument != node.Type && node.IsContainerBlock() {
		node = treenode.FirstLeafBlock(node)
	}
	content := sql.NodeStaticContent(node, nil, false, false, false)
	content = gulu.Str.SubStr(content, 128)
	content = strings.ReplaceAll(content, editor.Zwsp, "")
	err = SetCloudBlockReminder(id, content, timedMills)
	if err != nil {
		return
	}

	attrName := "custom-reminder-wechat"
	if "0" == timed {
		delete(attrs, attrName)
		old := node.IALAttr(attrName)
		oldTimedMills, e := dateparse.ParseIn(old, time.Now().Location())
		if nil == e {
			util.PushMsg(fmt.Sprintf(Conf.Language(109), oldTimedMills.Format("2006-01-02 15:04")), 3000)
		}
		node.RemoveIALAttr(attrName)
	} else {
		attrs[attrName] = timed
		node.SetIALAttr(attrName, timed)
		util.PushMsg(fmt.Sprintf(Conf.Language(101), time.UnixMilli(timedMills).Format("2006-01-02 15:04")), 5000)
	}
	if err = indexWriteTreeUpsertQueue(tree); err != nil {
		return
	}
	IncSync()
	cache.PutBlockIAL(id, attrs)
	return
}

func BatchSetBlockAttrs(blockAttrs []map[string]interface{}) (err error) {
	if util.ReadOnly {
		return
	}

	FlushTxQueue()

	var blockIDs []string
	for _, blockAttr := range blockAttrs {
		blockIDs = append(blockIDs, blockAttr["id"].(string))
	}

	trees := filesys.LoadTrees(blockIDs)
	var nodes []*ast.Node
	for _, blockAttr := range blockAttrs {
		id := blockAttr["id"].(string)
		tree := trees[id]
		if nil == tree {
			return errors.New(fmt.Sprintf(Conf.Language(15), id))
		}

		node := treenode.GetNodeInTree(tree, id)
		if nil == node {
			return errors.New(fmt.Sprintf(Conf.Language(15), id))
		}

		attrs := blockAttr["attrs"].(map[string]string)
		oldAttrs, e := setNodeAttrs0(node, attrs)
		if nil != e {
			return e
		}

		cache.PutBlockIAL(node.ID, parse.IAL2Map(node.KramdownIAL))
		pushBroadcastAttrTransactions(oldAttrs, node)
		nodes = append(nodes, node)
	}

	for _, tree := range trees {
		if err = indexWriteTreeUpsertQueue(tree); err != nil {
			return
		}
	}

	IncSync()
	// 不做锚文本刷新
	return
}

func SetBlockAttrs(id string, nameValues map[string]string) (err error) {
	if util.ReadOnly {
		return
	}

	FlushTxQueue()

	tree, err := LoadTreeByBlockID(id)
	if err != nil {
		return err
	}

	node := treenode.GetNodeInTree(tree, id)
	if nil == node {
		return errors.New(fmt.Sprintf(Conf.Language(15), id))
	}

	err = setNodeAttrs(node, tree, nameValues)
	return
}

func setNodeAttrs(node *ast.Node, tree *parse.Tree, nameValues map[string]string) (err error) {
	oldAttrs, err := setNodeAttrs0(node, nameValues)
	if err != nil {
		return
	}

	if err = indexWriteTreeUpsertQueue(tree); err != nil {
		return
	}

	IncSync()
	cache.PutBlockIAL(node.ID, parse.IAL2Map(node.KramdownIAL))

	pushBroadcastAttrTransactions(oldAttrs, node)

	go func() {
		sql.FlushQueue()
		refreshDynamicRefText(node, tree)
	}()
	return
}

func setNodeAttrsWithTx(tx *Transaction, node *ast.Node, tree *parse.Tree, nameValues map[string]string) (err error) {
	oldAttrs, err := setNodeAttrs0(node, nameValues)
	if err != nil {
		return
	}

	if err = tx.writeTree(tree); err != nil {
		return
	}

	IncSync()
	cache.PutBlockIAL(node.ID, parse.IAL2Map(node.KramdownIAL))
	pushBroadcastAttrTransactions(oldAttrs, node)
	return
}

func setNodeAttrs0(node *ast.Node, nameValues map[string]string) (oldAttrs map[string]string, err error) {
	oldAttrs = parse.IAL2Map(node.KramdownIAL)

	for name := range nameValues {
		for i := 0; i < len(name); i++ {
			if !lex.IsASCIILetterNumHyphen(name[i]) {
				err = errors.New(fmt.Sprintf(Conf.Language(25), node.ID))
				return
			}
		}
	}

	if tag, ok := nameValues["tags"]; ok {
		var tags []string
		tmp := strings.Split(tag, ",")
		for _, t := range tmp {
			t = util.RemoveInvalid(t)
			t = strings.TrimSpace(t)
			if "" != t {
				tags = append(tags, t)
			}
		}
		tags = gulu.Str.RemoveDuplicatedElem(tags)
		if 0 < len(tags) {
			nameValues["tags"] = strings.Join(tags, ",")
		}
	}

	for name, value := range nameValues {
		value = util.RemoveInvalidRetainCtrl(value)
		value = strings.TrimSpace(value)
		value = strings.TrimSuffix(value, ",")
		if "" == value {
			node.RemoveIALAttr(name)
		} else {
			node.SetIALAttr(name, value)
		}
	}

	if oldAttrs["tags"] != nameValues["tags"] {
		ReloadTag()
	}

	// Sync custom-* attribute changes to GA (reverse sync)
	syncCustomAttrsToGA(node.ID, oldAttrs, nameValues)
	return
}

func pushBroadcastAttrTransactions(oldAttrs map[string]string, node *ast.Node) {
	newAttrs := parse.IAL2Map(node.KramdownIAL)
	data := map[string]interface{}{"old": oldAttrs, "new": newAttrs}
	if "" != node.AttributeViewType {
		data["data-av-type"] = node.AttributeViewType
	}
	doOp := &Operation{Action: "updateAttrs", Data: data, ID: node.ID}
	evt := util.NewCmdResult("transactions", 0, util.PushModeBroadcast)
	evt.Data = []*Transaction{{
		DoOperations:   []*Operation{doOp},
		UndoOperations: []*Operation{},
	}}
	util.PushEvent(evt)
}

func ResetBlockAttrs(id string, nameValues map[string]string) (err error) {
	tree, err := LoadTreeByBlockID(id)
	if err != nil {
		return err
	}

	node := treenode.GetNodeInTree(tree, id)
	if nil == node {
		return errors.New(fmt.Sprintf(Conf.Language(15), id))
	}

	for name := range nameValues {
		for i := 0; i < len(name); i++ {
			if !lex.IsASCIILetterNumHyphen(name[i]) {
				return errors.New(fmt.Sprintf(Conf.Language(25), id))
			}
		}
	}

	node.ClearIALAttrs()
	for name, value := range nameValues {
		if "" != value {
			node.SetIALAttr(name, value)
		}
	}

	if ast.NodeDocument == node.Type {
		// 修改命名文档块后引用动态锚文本未跟随 https://github.com/siyuan-note/siyuan/issues/6398
		// 使用重命名文档队列来刷新引用锚文本
		updateRefTextRenameDoc(tree)
	}

	if err = indexWriteTreeUpsertQueue(tree); err != nil {
		return
	}
	IncSync()
	cache.RemoveBlockIAL(id)
	return
}

// syncCustomAttrsToGA syncs custom-* attribute changes from IAL to the corresponding GA.
// This implements the reverse sync: IAL → GA.
func syncCustomAttrsToGA(blockID string, oldAttrs, newAttrs map[string]string) {
	if blockID == "" {
		return
	}

	// Find all custom-* attributes that have changed
	for name, newValue := range newAttrs {
		if !strings.HasPrefix(name, "custom-") {
			continue
		}
		// Skip special custom attributes
		if name == "custom-avs" || name == "custom-gas" {
			continue
		}

		oldValue := oldAttrs[name]
		if oldValue == newValue {
			continue
		}

		// Extract the attribute name without "custom-" prefix
		attrName := strings.TrimPrefix(name, "custom-")
		if !IsValidCustomAttrName(attrName) {
			continue
		}

		// Find GA with this name that has isCustomAttr=true
		ga, err := FindCustomAttrGAByName(attrName)
		if err != nil {
			logging.LogWarnf("find custom attr GA by name [%s] failed: %s", attrName, err)
			continue
		}
		if ga == nil {
			// No GA bound to this custom attr, skip (Option A: ignore)
			continue
		}

		// Check if this block is bound to this GA (via custom-gas)
		boundGAs := newAttrs["custom-gas"]
		if boundGAs == "" {
			boundGAs = oldAttrs["custom-gas"]
		}
		isBound := false
		if boundGAs != "" {
			for _, gaID := range strings.Split(boundGAs, ",") {
				if gaID == ga.GaID {
					isBound = true
					break
				}
			}
		}
		if !isBound {
			// Block is not bound to this GA, skip reverse sync
			continue
		}

		// Sync the value to GA
		syncIALValueToGA(blockID, ga.GaID, newValue)
	}

	// Also check for removed custom-* attributes
	for name, oldValue := range oldAttrs {
		if !strings.HasPrefix(name, "custom-") {
			continue
		}
		if name == "custom-avs" || name == "custom-gas" {
			continue
		}
		if _, exists := newAttrs[name]; exists {
			continue
		}

		attrName := strings.TrimPrefix(name, "custom-")
		if !IsValidCustomAttrName(attrName) {
			continue
		}

		ga, err := FindCustomAttrGAByName(attrName)
		if err != nil || ga == nil {
			continue
		}

		boundGAs := newAttrs["custom-gas"]
		if boundGAs == "" {
			boundGAs = oldAttrs["custom-gas"]
		}
		isBound := false
		if boundGAs != "" {
			for _, gaID := range strings.Split(boundGAs, ",") {
				if gaID == ga.GaID {
					isBound = true
					break
				}
			}
		}
		if !isBound {
			continue
		}

		// Clear the value in GA
		syncIALValueToGA(blockID, ga.GaID, "")
		_ = oldValue // suppress unused warning
	}
}

// syncIALValueToGA syncs a single IAL value to the corresponding GA.
func syncIALValueToGA(blockID, gaID, ialValue string) {
	if blockID == "" || gaID == "" {
		return
	}

	attr, err := av.ParseGlobalAttribute(gaID)
	if err != nil {
		logging.LogWarnf("parse GA [%s] for IAL sync failed: %s", gaID, err)
		return
	}
	if attr == nil || attr.Key == nil {
		return
	}

	// Skip template and rollup - they don't support reverse sync
	if attr.Key.Type == av.KeyTypeTemplate || attr.Key.Type == av.KeyTypeRollup {
		return
	}

	// Find or create the value for this block
	var existingVal *av.Value
	for _, v := range attr.Values {
		if v != nil && v.BlockRefID == blockID {
			existingVal = v
			break
		}
	}

	now := util.CurrentTimeMillis()

	if ialValue == "" {
		// Clear the value
		if existingVal != nil {
			clearGAValue(existingVal)
			existingVal.SetUpdatedAt(now)
			if saveErr := av.SaveGlobalAttribute(attr); saveErr != nil {
				logging.LogWarnf("save GA [%s] after clearing value failed: %s", gaID, saveErr)
			} else {
				pushGlobalAttrKeyChanged(attr.Key)
			}
		}
		return
	}

	// Parse IAL value to GA value
	newVal := parseIALValueToGAValue(attr.Key, ialValue)
	if newVal == nil {
		return
	}

	if existingVal != nil {
		// Compare updatedAt to avoid conflicts - newer wins
		if existingVal.UpdatedAt >= now {
			// GA value is newer or same, skip
			return
		}
		// Update existing value
		copyGAValueContent(newVal, existingVal)
		existingVal.SetUpdatedAt(now)
	} else {
		// Create new value
		newVal.BlockRefID = blockID
		newVal.CreatedAt = now
		newVal.SetUpdatedAt(now)
		attr.Values = append(attr.Values, newVal)
	}

	if saveErr := av.SaveGlobalAttribute(attr); saveErr != nil {
		logging.LogWarnf("save GA [%s] after IAL sync failed: %s", gaID, saveErr)
	} else {
		pushGlobalAttrKeyChanged(attr.Key)
	}
}

// parseIALValueToGAValue parses an IAL string value to a GA value based on key type.
func parseIALValueToGAValue(key *av.Key, ialValue string) *av.Value {
	if key == nil {
		return nil
	}

	val := &av.Value{
		Type: key.Type,
	}

	switch key.Type {
	case av.KeyTypeText:
		val.Text = &av.ValueText{Content: ialValue}
	case av.KeyTypeNumber:
		num, err := strconv.ParseFloat(ialValue, 64)
		if err != nil {
			return nil
		}
		val.Number = &av.ValueNumber{Content: num, IsNotEmpty: true}
	case av.KeyTypeDate:
		// Parse date in format yyyyMMdd or yyyyMMddHHmmss
		var t time.Time
		var err error
		if len(ialValue) == 8 {
			t, err = time.ParseInLocation("20060102", ialValue, time.Local)
		} else if len(ialValue) == 14 {
			t, err = time.ParseInLocation("20060102150405", ialValue, time.Local)
		} else {
			return nil
		}
		if err != nil {
			return nil
		}
		val.Date = &av.ValueDate{
			Content:   t.UnixMilli(),
			IsNotTime: len(ialValue) == 8,
		}
	case av.KeyTypeSelect:
		if ialValue != "" {
			color := getOrCreateOptionColor(key, ialValue)
			val.MSelect = []*av.ValueSelect{{Content: ialValue, Color: color}}
		}
	case av.KeyTypeMSelect:
		parts := strings.Split(ialValue, ",")
		var selects []*av.ValueSelect
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				color := getOrCreateOptionColor(key, part)
				selects = append(selects, &av.ValueSelect{Content: part, Color: color})
			}
		}
		val.MSelect = selects
	case av.KeyTypeURL:
		val.URL = &av.ValueURL{Content: ialValue}
	case av.KeyTypeEmail:
		val.Email = &av.ValueEmail{Content: ialValue}
	case av.KeyTypePhone:
		val.Phone = &av.ValuePhone{Content: ialValue}
	case av.KeyTypeCheckbox:
		val.Checkbox = &av.ValueCheckbox{Checked: ialValue == "true"}
	case av.KeyTypeRelation:
		// Relation reverse sync not supported in this phase
		return nil
	default:
		return nil
	}

	return val
}

// getOrCreateOptionColor returns the color for an option, creating a new option if needed.
func getOrCreateOptionColor(key *av.Key, optionName string) string {
	if key == nil {
		return "1"
	}
	for _, opt := range key.Options {
		if opt != nil && opt.Name == optionName {
			return opt.Color
		}
	}
	// Create new option with auto color
	newColor := fmt.Sprintf("%d", (len(key.Options)%14)+1)
	key.Options = append(key.Options, &av.SelectOption{Name: optionName, Color: newColor})
	return newColor
}

// clearGAValue clears all value slots of a GA value.
func clearGAValue(val *av.Value) {
	if val == nil {
		return
	}
	val.Text = nil
	val.Number = nil
	val.Date = nil
	val.MSelect = nil
	val.URL = nil
	val.Email = nil
	val.Phone = nil
	val.Checkbox = nil
	val.Relation = nil
}

// copyGAValueContent copies the content from src to dst.
func copyGAValueContent(src, dst *av.Value) {
	if src == nil || dst == nil {
		return
	}
	dst.Text = src.Text
	dst.Number = src.Number
	dst.Date = src.Date
	dst.MSelect = src.MSelect
	dst.URL = src.URL
	dst.Email = src.Email
	dst.Phone = src.Phone
	dst.Checkbox = src.Checkbox
	dst.Relation = src.Relation
}
