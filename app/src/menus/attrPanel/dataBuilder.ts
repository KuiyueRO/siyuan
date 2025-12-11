import {IAttrViewTableResponse} from "../../protyle/render/av/blockAttr";
import {BUILTIN_ATTR_VIEW_ID, isBuiltinGlobalAttrId, isWritableBuiltinGlobalAttrId} from "../../protyle/render/av/globalAttr";
import {
    IAttrPanelCustomAttrValue,
    IAttrPanelDedupItem,
    IAttrPanelDedupSource,
    IAttrPanelFieldInstance,
    IAttrPanelViewBinding,
    TAttrPanelFieldScope
} from "../../types/attrPanel";

const getBuiltinLabel = (languages: IObject) => languages.blockAttrBuiltin || languages.globalAttr || languages.builtinAttrReadonly || "内置";
const getDatabaseLabel = (languages: IObject) => languages.database || "数据库";
const getCustomLabel = (languages: IObject) => languages.custom || languages.attrCustom || "自定义";

export const buildAttrPanelViewBindingsFromTables = (tables: IAttrViewTableResponse[] = [], languages: IObject): IAttrPanelViewBinding[] => {
    return tables.map(table => {
        const isBuiltinView = table.avID === BUILTIN_ATTR_VIEW_ID;
        const scope: TAttrPanelFieldScope = isBuiltinView ? "builtin" : "database";
        const avName = table.avName || (isBuiltinView ? getBuiltinLabel(languages) : getDatabaseLabel(languages));
        const fields: IAttrPanelFieldInstance[] = [];
        table.keyValues?.forEach(pair => {
            if (!pair?.key || !pair.values || pair.values.length === 0 || !pair.key.type) {
                return;
            }
            const firstValue = pair.values[0];
            const gaId = pair.key.gaId || "";
            const builtinGa = gaId ? isBuiltinGlobalAttrId(gaId) : false;
            const builtinWritable = builtinGa ? isWritableBuiltinGlobalAttrId(gaId) : false;
            const readonlyType = firstValue?.type ? ["created", "updated"].includes(firstValue.type) : false;
            fields.push({
                keyId: pair.key.id,
                gaId,
                name: pair.key.name || pair.key.id,
                desc: pair.key.desc,
                icon: pair.key.icon,
                type: pair.key.type,
                scope,
                isCustomAttr: !!pair.key.isCustomAttr,
                readonly: builtinGa ? !builtinWritable : readonlyType,
                builtinWritable,
                value: firstValue,
            });
        });
        return {
            avId: table.avID,
            avName,
            isBuiltin: isBuiltinView,
            blockIds: table.blockIDs || [],
            fields
        };
    });
};

export const buildAttrPanelDedupItems = (
    viewBindings: IAttrPanelViewBinding[],
    customEntries: IAttrPanelCustomAttrValue[],
    languages: IObject
): IAttrPanelDedupItem[] => {
    const builtinLabel = getBuiltinLabel(languages);
    const customLabel = getCustomLabel(languages);
    const summaryMap = new Map<string, IAttrPanelDedupItem>();
    const ensureSummary = (id: string, name: string, opts: {desc?: string; icon?: string; isCustomAttr?: boolean; builtinWritable?: boolean}) => {
        if (!summaryMap.has(id)) {
            summaryMap.set(id, {
                id,
                name,
                desc: opts.desc,
                icon: opts.icon,
                isCustomAttr: !!opts.isCustomAttr,
                builtinWritable: opts.builtinWritable,
                sources: []
            });
        }
        const summary = summaryMap.get(id)!;
        if (!summary.desc && opts.desc) {
            summary.desc = opts.desc;
        }
        if (!summary.icon && opts.icon) {
            summary.icon = opts.icon;
        }
        if (opts.builtinWritable !== undefined) {
            summary.builtinWritable = opts.builtinWritable;
        }
        if (opts.isCustomAttr) {
            summary.isCustomAttr = true;
        }
        return summary;
    };

    viewBindings.forEach(binding => {
        const sectionLabel = binding.isBuiltin ? builtinLabel : (binding.avName || getDatabaseLabel(languages));
        binding.fields?.forEach(field => {
            const fallbackId = binding.isBuiltin ? field.keyId : `${binding.avId}:${field.keyId}`;
            const summaryId = field.gaId || fallbackId;
            const summary = ensureSummary(summaryId, field.name || summaryId, {
                desc: field.desc,
                icon: field.icon,
                isCustomAttr: field.isCustomAttr,
                builtinWritable: field.builtinWritable
            });
            summary.sources.push({
                label: sectionLabel,
                scope: field.scope,
                targetType: "av",
                avId: binding.avId,
                keyId: field.keyId
            });
        });
    });

    customEntries?.forEach(entry => {
        if (!entry) {
            return;
        }
        const summaryId = `custom:${entry.name}`;
        const summary = ensureSummary(summaryId, entry.label || entry.name.replace(/^custom-/i, ""), {
            isCustomAttr: true
        });
        summary.sources.push({
            label: customLabel,
            scope: "custom",
            targetType: "custom",
            customName: entry.name
        });
    });

    return Array.from(summaryMap.values());
};
