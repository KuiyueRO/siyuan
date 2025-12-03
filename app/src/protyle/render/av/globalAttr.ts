import {fetchPost} from "../../../util/fetch";

export const BUILTIN_ATTR_VIEW_ID = "__builtin_global_attrs__";

// Keep this list in sync with kernel/model/globalattr_builtin.go
const builtinGlobalAttrIdSet = new Set([
    "alias",
    "box",
    "bookmark",
    "content",
    "created",
    "fcontent",
    "hash",
    "hPath",
    "ial",
    "id",
    "length",
    "markdown",
    "memo",
    "name",
    "parentId",
    "path",
    "rootId",
    "sort",
    "subType",
    "tag",
    "type",
    "updated",
]);

// Keep this list in sync with kernel/model/globalattr_builtin.go writable specs
const writableBuiltinGlobalAttrIdSet = new Set([
    "alias",
    "bookmark",
    "memo",
    "name",
    "tag",
]);

export const isBuiltinGlobalAttrId = (gaId?: string | null) => {
    if (!gaId) {
        return false;
    }
    return builtinGlobalAttrIdSet.has(gaId);
};

export const isWritableBuiltinGlobalAttrId = (gaId?: string | null) => {
    if (!gaId) {
        return false;
    }
    return writableBuiltinGlobalAttrIdSet.has(gaId);
};

export interface IGlobalAttrMeta {
    gaId: string;
    name: string;
    icon?: string;
    desc?: string;
    type: string;
    options?: {
        name: string,
        color: string,
        desc?: string,
    }[];
    numberFormat?: string;
    template?: string;
    isCustomAttr?: boolean;
}

let cachedAttrs: IGlobalAttrMeta[] | null = null;
let inflight: Promise<IGlobalAttrMeta[]> | null = null;

export const getGlobalAttrs = (): Promise<IGlobalAttrMeta[]> => {
    if (cachedAttrs) {
        return Promise.resolve(cachedAttrs);
    }
    if (inflight) {
        return inflight;
    }
    inflight = new Promise((resolve, reject) => {
        fetchPost("/api/globalattr/list", {}, (response) => {
            inflight = null;
            if (!response || response.code !== 0) {
                reject(response?.msg || "Failed to load global attributes");
                return;
            }
            const attrs = (response.data?.attrs || []) as IGlobalAttrMeta[];
            cachedAttrs = attrs;
            resolve(attrs);
        });
    });
    return inflight;
};

export const invalidateGlobalAttrCache = () => {
    cachedAttrs = null;
    inflight = null;
};

/**
 * Checks if a name is valid for custom attribute usage.
 * Valid names must start with an ASCII letter and contain only ASCII letters, numbers, and hyphens.
 */
export const isValidCustomAttrName = (name: string): boolean => {
    if (!name || name.length === 0) {
        return false;
    }
    // First character must be a letter
    const first = name.charCodeAt(0);
    if (!((first >= 65 && first <= 90) || (first >= 97 && first <= 122))) {
        return false;
    }
    // Subsequent characters can be letters, numbers, or hyphens
    for (let i = 1; i < name.length; i++) {
        const c = name.charCodeAt(i);
        if (!((c >= 65 && c <= 90) || (c >= 97 && c <= 122) || (c >= 48 && c <= 57) || c === 45)) {
            return false;
        }
    }
    return true;
};

/**
 * Finds a global attribute with the given name that has isCustomAttr=true.
 * Returns null if no such attribute exists.
 */
export const findCustomAttrGAByName = async (name: string): Promise<IGlobalAttrMeta | null> => {
    const attrs = await getGlobalAttrs();
    for (const attr of attrs) {
        if (attr.isCustomAttr && attr.name === name) {
            return attr;
        }
    }
    return null;
};

/**
 * Checks if there's another GA with the same name that already has isCustomAttr=true.
 * Returns the conflicting GA if found, null otherwise.
 */
export const checkCustomAttrNameConflict = async (name: string, excludeGaId?: string): Promise<IGlobalAttrMeta | null> => {
    const attrs = await getGlobalAttrs();
    for (const attr of attrs) {
        if (attr.isCustomAttr && attr.name === name && attr.gaId !== excludeGaId) {
            return attr;
        }
    }
    return null;
};
