package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/88250/lute/ast"
	"github.com/siyuan-note/siyuan/kernel/av"
	"github.com/siyuan-note/siyuan/kernel/sql"
)

type builtinAttrHydrator func(val *av.Value, block *sql.Block, attrs map[string]string)
type builtinAttrWriter func(tx *Transaction, blockID string, val *av.Value) error
type builtinAttrOptionsProvider func() []*av.SelectOption

type builtinAttrSpec struct {
	attr            *GlobalAttr
	icon            string
	hydrate         builtinAttrHydrator
	write           builtinAttrWriter
	optionsProvider builtinAttrOptionsProvider
}

var (
	builtinAttrSpecs   []*builtinAttrSpec
	builtinAttrSpecMap map[string]*builtinAttrSpec
)

func init() {
	builtinAttrSpecs = []*builtinAttrSpec{
		newBuiltinAttrSpec("id", "ID", "", av.KeyTypeText, "块 ID", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.ID)
		}),
		newBuiltinAttrSpec("parentId", "Parent ID", "", av.KeyTypeText, "父块 ID", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.ParentID)
		}),
		newBuiltinAttrSpec("rootId", "Root ID", "", av.KeyTypeText, "根块 ID", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.RootID)
		}),
		newBuiltinAttrSpec("hash", "Hash", "", av.KeyTypeText, "内容哈希", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Hash)
		}),
		newBuiltinAttrSpec("box", "Box", "", av.KeyTypeText, "笔记本盒子", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Box)
		}),
		newBuiltinAttrSpec("path", "Path", "", av.KeyTypeText, "文档路径", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Path)
		}),
		newBuiltinAttrSpec("hPath", "HPath", "", av.KeyTypeText, "人类可读路径", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.HPath)
		}),
		newBuiltinAttrSpec("name", "Name", "iconN", av.KeyTypeSelect, "名称", func(val *av.Value, block *sql.Block, attrs map[string]string) {
			fillSelectValue(val, attrOrDefault(attrs, "name", block.Name))
		}, builtinSelectAttrWriter("name")),
		newBuiltinAttrSpec("alias", "Alias", "iconA", av.KeyTypeMSelect, "别名", func(val *av.Value, block *sql.Block, attrs map[string]string) {
			fillMSelectValue(val, splitCSV(attrOrDefault(attrs, "alias", block.Alias)))
		}, builtinMultiSelectAttrWriter("alias", false)),
		newBuiltinAttrSpec("memo", "Memo", "iconM", av.KeyTypeText, "备注", func(val *av.Value, block *sql.Block, attrs map[string]string) {
			fillTextValue(val, attrOrDefault(attrs, "memo", block.Memo))
		}, builtinTextAttrWriter("memo")),
		withBuiltinOptions(newBuiltinAttrSpec("bookmark", "Bookmark", "iconBookmark", av.KeyTypeSelect, "书签", func(val *av.Value, _ *sql.Block, attrs map[string]string) {
			fillSelectValue(val, attrs["bookmark"])
		}, builtinSelectAttrWriter("bookmark")), bookmarkSelectOptions),
		newBuiltinAttrSpec("tag", "Tag", "iconTags", av.KeyTypeMSelect, "标签", func(val *av.Value, _ *sql.Block, attrs map[string]string) {
			fillMSelectValue(val, splitCSV(attrs["tags"]))
		}, builtinMultiSelectAttrWriter("tags", true)),
		newBuiltinAttrSpec("content", "Content", "", av.KeyTypeText, "原始内容", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Content)
		}),
		newBuiltinAttrSpec("fcontent", "FContent", "", av.KeyTypeText, "格式化内容", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.FContent)
		}),
		newBuiltinAttrSpec("markdown", "Markdown", "", av.KeyTypeText, "Markdown 内容", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Markdown)
		}),
		newBuiltinAttrSpec("length", "Length", "", av.KeyTypeNumber, "内容长度", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillNumberValue(val, float64(block.Length), block.Length != 0)
		}),
		newBuiltinAttrSpec("type", "Type", "", av.KeyTypeText, "块类型", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Type)
		}),
		newBuiltinAttrSpec("subType", "Subtype", "", av.KeyTypeText, "子类型", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.SubType)
		}),
		newBuiltinAttrSpec("ial", "IAL", "", av.KeyTypeText, "属性 IAL", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.IAL)
		}),
		newBuiltinAttrSpec("sort", "Sort", "", av.KeyTypeNumber, "排序值", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillNumberValue(val, float64(block.Sort), true)
		}),
		newBuiltinAttrSpec("created", "Created", "", av.KeyTypeText, "创建时间", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Created)
		}),
		newBuiltinAttrSpec("updated", "Updated", "", av.KeyTypeText, "更新时间", func(val *av.Value, block *sql.Block, _ map[string]string) {
			fillTextValue(val, block.Updated)
		}),
	}

	builtinAttrSpecMap = map[string]*builtinAttrSpec{}
	for _, spec := range builtinAttrSpecs {
		builtinAttrSpecMap[spec.attr.GaID] = spec
	}
}

func newBuiltinAttrSpec(id, name, icon string, keyType av.KeyType, desc string, hydrator builtinAttrHydrator, writer ...builtinAttrWriter) *builtinAttrSpec {
	attr := newBuiltinGlobalAttr(id, name, keyType, desc)
	attr.Icon = icon
	spec := &builtinAttrSpec{attr: attr, icon: icon, hydrate: hydrator}
	if len(writer) > 0 {
		spec.write = writer[0]
	}
	return spec
}

func withBuiltinOptions(spec *builtinAttrSpec, provider builtinAttrOptionsProvider) *builtinAttrSpec {
	if spec != nil {
		spec.optionsProvider = provider
	}
	return spec
}

func bookmarkSelectOptions() []*av.SelectOption {
	labels := BookmarkLabels()
	if len(labels) == 0 {
		return nil
	}
	options := make([]*av.SelectOption, 0, len(labels))
	for idx, label := range labels {
		options = append(options, &av.SelectOption{Name: label, Color: nextAutoColor(idx)})
	}
	return options
}

func collectBuiltinGlobalAttrs() []*GlobalAttr {
	attrs := make([]*GlobalAttr, 0, len(builtinAttrSpecs))
	for _, spec := range builtinAttrSpecs {
		attrs = append(attrs, spec.attr)
	}
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].GaID < attrs[j].GaID
	})
	return attrs
}

func hydrateBuiltinGlobalAttrValues(attrView *av.AttributeView) {
	if attrView == nil {
		return
	}
	blockKeyValues := attrView.GetBlockKeyValues()
	if blockKeyValues == nil {
		return
	}

	type binding struct {
		rowID   string
		blockID string
	}

	var (
		bindings       []binding
		uniqueBlockIDs []string
		blockIDSet     = map[string]struct{}{}
	)
	for _, blockVal := range blockKeyValues.Values {
		if blockVal == nil || blockVal.IsDetached {
			continue
		}

		rowID := blockVal.BlockID
		if rowID == "" {
			continue
		}

		blockID := blockVal.BlockRefID
		if blockID == "" && blockVal.Block != nil {
			blockID = blockVal.Block.ID
		}
		if blockID == "" {
			continue
		}

		bindings = append(bindings, binding{rowID: rowID, blockID: blockID})
		if _, ok := blockIDSet[blockID]; !ok {
			blockIDSet[blockID] = struct{}{}
			uniqueBlockIDs = append(uniqueBlockIDs, blockID)
		}
	}
	if len(bindings) == 0 {
		return
	}

	blocks := sql.GetBlocks(uniqueBlockIDs)
	if len(blocks) == 0 {
		return
	}
	blockMap := make(map[string]*sql.Block, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		blockMap[block.ID] = block
	}
	attrsMap := sql.BatchGetBlockAttrs(uniqueBlockIDs)

	for _, keyValues := range attrView.KeyValues {
		spec := builtinAttrSpecMap[keyValues.Key.GaID]
		if spec == nil {
			continue
		}
		ensureBuiltinKeyMetadata(keyValues.Key, spec)
		for _, binding := range bindings {
			block := blockMap[binding.blockID]
			if block == nil {
				continue
			}
			attrs := attrsMap[binding.blockID]
			val := keyValues.GetValue(binding.rowID)
			if val == nil {
				val = &av.Value{
					ID:               ast.NewNodeID(),
					KeyID:            keyValues.Key.ID,
					BlockID:          binding.rowID,
					BlockRefID:       binding.blockID,
					Type:             keyValues.Key.Type,
					IsRenderAutoFill: true,
				}
				keyValues.Values = append(keyValues.Values, val)
			} else {
				val.BlockID = binding.rowID
				val.BlockRefID = binding.blockID
				val.IsRenderAutoFill = true
				val.Type = keyValues.Key.Type
			}
			spec.hydrate(val, block, attrs)
		}
	}
}

func ensureBuiltinKeyMetadata(key *av.Key, spec *builtinAttrSpec) {
	if key == nil || spec == nil || spec.attr == nil {
		return
	}
	attr := spec.attr
	key.Type = attr.Type
	if attr.Icon != "" {
		key.Icon = attr.Icon
	}
	key.Desc = attr.Desc
	key.Name = attr.Name
	if spec.optionsProvider != nil && (key.Type == av.KeyTypeSelect || key.Type == av.KeyTypeMSelect) {
		if options := spec.optionsProvider(); len(options) > 0 {
			key.Options = options
		}
	}
}

func fillTextValue(val *av.Value, content string) {
	if val == nil {
		return
	}
	resetValueSlots(val)
	if val.Text == nil {
		val.Text = &av.ValueText{}
	}
	val.Type = av.KeyTypeText
	val.Text.Content = content
}

func fillNumberValue(val *av.Value, number float64, isNotEmpty bool) {
	if val == nil {
		return
	}
	resetValueSlots(val)
	if val.Number == nil {
		val.Number = &av.ValueNumber{}
	}
	val.Type = av.KeyTypeNumber
	val.Number.Content = number
	val.Number.IsNotEmpty = isNotEmpty
	if isNotEmpty {
		val.Number.FormattedContent = strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%f", number), "0"), ".")
	} else {
		val.Number.FormattedContent = ""
	}
}

func fillMSelectValue(val *av.Value, values []string) {
	if val == nil {
		return
	}
	resetValueSlots(val)
	if len(values) == 0 {
		val.MSelect = nil
		val.Type = av.KeyTypeMSelect
		return
	}
	var selects []*av.ValueSelect
	seen := map[string]struct{}{}
	colorIndex := 0
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		selects = append(selects, &av.ValueSelect{Content: v, Color: nextAutoColor(colorIndex)})
		colorIndex++
	}
	val.Type = av.KeyTypeMSelect
	val.MSelect = selects
}

func fillSelectValue(val *av.Value, value string) {
	if val == nil {
		return
	}
	resetValueSlots(val)
	val.Type = av.KeyTypeSelect
	value = strings.TrimSpace(value)
	if value == "" {
		val.MSelect = nil
		return
	}
	val.MSelect = []*av.ValueSelect{{Content: value, Color: nextAutoColor(0)}}
}

func resetValueSlots(val *av.Value) {
	val.Block = nil
	val.Text = nil
	val.Number = nil
	val.Date = nil
	val.MSelect = nil
	val.URL = nil
	val.Email = nil
	val.Phone = nil
	val.MAsset = nil
	val.Template = nil
	val.Created = nil
	val.Updated = nil
	val.Checkbox = nil
	val.Relation = nil
	val.Rollup = nil
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var ret []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ret = append(ret, part)
	}
	return ret
}

func attrOrDefault(attrs map[string]string, key, fallback string) string {
	if attrs != nil {
		if v, ok := attrs[key]; ok && v != "" {
			return v
		}
	}
	return fallback
}

func builtinTextAttrWriter(attrName string) builtinAttrWriter {
	return func(tx *Transaction, blockID string, val *av.Value) error {
		var content string
		if val != nil && val.Text != nil {
			content = strings.TrimSpace(val.Text.Content)
		}
		return writeBlockAttrs(tx, blockID, map[string]string{attrName: content})
	}
}

func builtinSelectAttrWriter(attrName string) builtinAttrWriter {
	return func(tx *Transaction, blockID string, val *av.Value) error {
		var content string
		if val != nil && len(val.MSelect) > 0 && val.MSelect[0] != nil {
			content = strings.TrimSpace(val.MSelect[0].Content)
		}
		return writeBlockAttrs(tx, blockID, map[string]string{attrName: content})
	}
}

func builtinMultiSelectAttrWriter(attrName string, docOnly bool) builtinAttrWriter {
	return func(tx *Transaction, blockID string, val *av.Value) error {
		if docOnly {
			block := sql.GetBlock(blockID)
			if block == nil || block.Type != "d" {
				return fmt.Errorf("global attribute %s is only writable for document blocks", attrName)
			}
		}
		seen := map[string]struct{}{}
		var parts []string
		if val != nil {
			for _, opt := range val.MSelect {
				if opt == nil {
					continue
				}
				text := strings.TrimSpace(opt.Content)
				if text == "" {
					continue
				}
				if _, ok := seen[text]; ok {
					continue
				}
				seen[text] = struct{}{}
				parts = append(parts, text)
			}
		}
		var content string
		if len(parts) > 0 {
			content = strings.Join(parts, ", ")
		}
		return writeBlockAttrs(tx, blockID, map[string]string{attrName: content})
	}
}

func writeBlockAttrs(tx *Transaction, blockID string, attrs map[string]string) error {
	if blockID == "" {
		return fmt.Errorf("block id is empty")
	}
	if tx != nil {
		node, tree, err := getNodeByBlockID(tx, blockID)
		if err != nil {
			return err
		}
		if node == nil || tree == nil {
			return fmt.Errorf("block %s not found", blockID)
		}
		return setNodeAttrsWithTx(tx, node, tree, attrs)
	}
	return SetBlockAttrs(blockID, attrs)
}

func nextAutoColor(index int) string {
	const paletteSize = 14
	return fmt.Sprintf("%d", (index%paletteSize)+1)
}
