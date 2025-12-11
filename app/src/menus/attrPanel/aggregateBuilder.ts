import {IAttrViewTableResponse} from "../../protyle/render/av/blockAttr";
import {
    IAttrPanelAggregateResponse,
    IAttrPanelCustomAttrValue,
    IAttrPanelDedupItem,
    IAttrPanelFieldInstance,
    IAttrPanelGlobalAttrSummary,
    IAttrPanelServerPreferences,
    IAttrPanelViewBinding
} from "../../types/attrPanel";
import {buildAttrPanelDedupItems, buildAttrPanelViewBindingsFromTables} from "./dataBuilder";

const buildGlobalAttrCatalog = (dedupItems: IAttrPanelDedupItem[]): IAttrPanelGlobalAttrSummary[] => {
    return dedupItems.map(item => {
        const boundAvIds = new Set<string>();
        item.sources.forEach(source => {
            if (source.targetType === "av" && source.avId) {
                boundAvIds.add(source.avId);
            }
        });
        const isBuiltin = item.sources.some(source => source.scope === "builtin");
        return {
            gaId: item.id.startsWith("custom:") ? undefined : item.id,
            name: item.name,
            desc: item.desc,
            icon: item.icon,
            isCustomAttr: item.isCustomAttr,
            isBuiltin,
            usageCount: item.sources.length,
            boundAvIds: Array.from(boundAvIds.values())
        };
    });
};

const collectBuiltinFields = (viewBindings: IAttrPanelViewBinding[]): IAttrPanelFieldInstance[] => {
    const fields: IAttrPanelFieldInstance[] = [];
    viewBindings.forEach(binding => {
        if (binding.isBuiltin && binding.fields) {
            fields.push(...binding.fields);
        }
    });
    return fields;
};

export interface IAttrPanelAggregateOptions {
    preferences?: IAttrPanelServerPreferences;
    prebuiltViewBindings?: IAttrPanelViewBinding[];
    prebuiltDedupItems?: IAttrPanelDedupItem[];
}

export const buildAttrPanelAggregateResponse = (
    blockId: string,
    tables: IAttrViewTableResponse[] = [],
    customEntries: IAttrPanelCustomAttrValue[] = [],
    languages: IObject,
    options: IAttrPanelAggregateOptions = {}
): IAttrPanelAggregateResponse => {
    const viewBindings = options.prebuiltViewBindings || buildAttrPanelViewBindingsFromTables(tables, languages);
    const dedupItems = options.prebuiltDedupItems || buildAttrPanelDedupItems(viewBindings, customEntries, languages);
    const builtinFields = collectBuiltinFields(viewBindings);
    const globalAttrCatalog = buildGlobalAttrCatalog(dedupItems);
    return {
        blockId,
        fetchedAt: Date.now(),
        builtinFields,
        viewBindings,
        customAttrs: customEntries,
        globalAttrCatalog,
        dedupItems,
        preferences: options.preferences
    };
};
