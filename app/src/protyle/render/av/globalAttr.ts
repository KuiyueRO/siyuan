import {fetchPost} from "../../../util/fetch";

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
