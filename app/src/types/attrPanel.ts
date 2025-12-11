export type TAttrPanelMode = "grouped" | "dedup";

export type TAttrPanelSortKey =
    | "custom"
    | "nameAsc"
    | "nameDesc"
    | "createdAsc"
    | "createdDesc"
    | "updatedAsc"
    | "updatedDesc";

export interface IAttrPanelPreferences {
    mode: TAttrPanelMode;
    sort: TAttrPanelSortKey;
    showReadonlyBuiltin: boolean;
    customOrder: Record<string, string[]>;
}

/**
 * Server-provided defaults that hydrate the client before local overrides apply.
 */
export interface IAttrPanelServerPreferences {
    mode?: TAttrPanelMode;
    sort?: TAttrPanelSortKey;
    showReadonlyBuiltin?: boolean;
    customOrder?: Record<string, string[]>;
}

export type TAttrPanelFieldScope = "builtin" | "database" | "custom";

export type TAttrPanelFieldTarget = "av" | "custom";

export interface IAttrPanelFieldInstance {
    keyId: string;
    gaId?: string;
    name: string;
    desc?: string;
    icon?: string;
    type: TAVCol;
    scope: TAttrPanelFieldScope;
    isCustomAttr?: boolean;
    readonly: boolean;
    builtinWritable?: boolean;
    value: IAVCellValue;
    createdAt?: number;
    updatedAt?: number;
}

export interface IAttrPanelViewBinding {
    avId: string;
    avName: string;
    isBuiltin: boolean;
    blockIds: string[];
    fields: IAttrPanelFieldInstance[];
}

export interface IAttrPanelCustomAttrValue {
    name: string;
    label: string;
    value: string;
    createdAt?: number;
    updatedAt?: number;
}

export interface IAttrPanelDedupSource {
    label: string;
    scope: TAttrPanelFieldScope;
    targetType: TAttrPanelFieldTarget;
    avId?: string;
    keyId?: string;
    customName?: string;
}

export interface IAttrPanelDedupItem {
    id: string;
    name: string;
    desc?: string;
    icon?: string;
    isCustomAttr: boolean;
    builtinWritable?: boolean;
    sources: IAttrPanelDedupSource[];
}

export interface IAttrPanelGlobalAttrSummary {
    gaId?: string;
    name: string;
    desc?: string;
    icon?: string;
    isCustomAttr: boolean;
    isBuiltin: boolean;
    usageCount: number;
    boundAvIds: string[];
    createdAt?: number;
    updatedAt?: number;
}

export interface IAttrPanelAggregateResponse {
    blockId: string;
    fetchedAt: number;
    builtinFields: IAttrPanelFieldInstance[];
    viewBindings: IAttrPanelViewBinding[];
    customAttrs: IAttrPanelCustomAttrValue[];
    globalAttrCatalog: IAttrPanelGlobalAttrSummary[];
    dedupItems: IAttrPanelDedupItem[];
    preferences?: IAttrPanelServerPreferences;
}
