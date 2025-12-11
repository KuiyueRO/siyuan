/// #if !BROWSER
import {shell} from "electron";
/// #endif
import {confirmDialog} from "../dialog/confirmDialog";
import {getSearch, isMobile, isValidAttrName} from "../util/functions";
import {isLocalPath, movePathTo, moveToPath, pathPosix} from "../util/pathName";
import {MenuItem} from "./Menu";
import {onExport, saveExport} from "../protyle/export";
import {isInAndroid, isInHarmony, openByMobile, writeText, setStorageVal} from "../protyle/util/compatibility";
import {fetchPost, fetchSyncPost} from "../util/fetch";
import {hideMessage, showMessage} from "../dialog/message";
import {Dialog} from "../dialog";
import {focusBlock, focusByRange, getEditorRange} from "../protyle/util/selection";
/// #if !MOBILE
import {openAsset, openBy} from "../editor/util";
/// #endif
import {rename, replaceFileName} from "../editor/rename";
import * as dayjs from "dayjs";
import {Constants} from "../constants";
import {exportImage} from "../protyle/export/util";
import {App} from "../index";
import {renderAVAttribute, IAttrViewTableResponse} from "../protyle/render/av/blockAttr";
import {addEditorToDatabase} from "../protyle/render/av/addToDatabase";
import {openAssetNewWindow} from "../window/openNewWindow";
import {escapeAttr, escapeHtml} from "../util/escape";
import {copyTextByType} from "../protyle/toolbar/util";
import {hideElements} from "../protyle/ui/hideElements";
import {Protyle} from "../protyle";
import {getAllEditor} from "../layout/getAll";
import {Menu as PopupMenu} from "../plugin/Menu";
import {BUILTIN_ATTR_VIEW_ID} from "../protyle/render/av/globalAttr";
import {
    IAttrPanelAggregateResponse,
    IAttrPanelCustomAttrValue,
    IAttrPanelDedupItem,
    IAttrPanelDedupSource,
    IAttrPanelPreferences,
    IAttrPanelViewBinding,
    TAttrPanelFieldTarget,
    TAttrPanelMode,
    TAttrPanelSortKey
} from "../types/attrPanel";
import {buildAttrPanelDedupItems, buildAttrPanelViewBindingsFromTables} from "./attrPanel/dataBuilder";
import {buildAttrPanelAggregateResponse} from "./attrPanel/aggregateBuilder";

const bindAttrInput = (inputElement: HTMLInputElement | HTMLTextAreaElement, id: string) => {
    if (!inputElement) {
        return;
    }
    const attrName = inputElement.dataset.name;
    if (!attrName) {
        return;
    }

    const resizeTextarea = () => {
        if (inputElement instanceof HTMLTextAreaElement) {
            inputElement.style.height = "auto";
            inputElement.style.height = `${inputElement.scrollHeight}px`;
        }
    };
    resizeTextarea();

    let lastSavedValue = inputElement.value;
    let debounceId = 0;
    const persistValue = () => {
        const currentValue = inputElement.value;
        if (currentValue === lastSavedValue) {
            return;
        }
        lastSavedValue = currentValue;
        fetchPost("/api/attr/setBlockAttrs", {
            id,
            attrs: {[attrName]: currentValue}
        });
    };
    const schedulePersist = () => {
        if (debounceId) {
            window.clearTimeout(debounceId);
        }
        debounceId = window.setTimeout(persistValue, 400);
    };

    inputElement.addEventListener("input", () => {
        resizeTextarea();
        schedulePersist();
    });
    inputElement.addEventListener("blur", () => {
        if (debounceId) {
            window.clearTimeout(debounceId);
        }
        persistValue();
    });
};

const ATTR_PANEL_SORT_VALUES: TAttrPanelSortKey[] = ["custom", "nameAsc", "nameDesc", "createdAsc", "createdDesc", "updatedAsc", "updatedDesc"];
const ATTR_PANEL_SORT_GROUPS: TAttrPanelSortKey[][] = [["custom"], ["nameAsc", "nameDesc"], ["createdAsc", "createdDesc"], ["updatedAsc", "updatedDesc"]];

const createAttrPanelPreferenceDefaults = (): IAttrPanelPreferences => ({
    mode: "grouped",
    sort: "custom",
    showReadonlyBuiltin: false,
    customOrder: {}
});

const ensureAttrPanelPreferences = (): IAttrPanelPreferences => {
    if (!window.siyuan.storage) {
        window.siyuan.storage = {} as IObject;
    }
    const stored = window.siyuan.storage[Constants.LOCAL_ATTR_PANEL];
    if (!stored || typeof stored !== "object") {
        const defaults = createAttrPanelPreferenceDefaults();
        window.siyuan.storage[Constants.LOCAL_ATTR_PANEL] = defaults;
        setStorageVal(Constants.LOCAL_ATTR_PANEL, defaults);
        return defaults;
    }
    if (stored.mode !== "grouped" && stored.mode !== "dedup") {
        stored.mode = "grouped";
    }
    if (!ATTR_PANEL_SORT_VALUES.includes(stored.sort)) {
        stored.sort = "custom";
    }
    if (typeof stored.showReadonlyBuiltin !== "boolean") {
        stored.showReadonlyBuiltin = false;
    }
    if (!stored.customOrder || typeof stored.customOrder !== "object") {
        stored.customOrder = {};
    }
    return stored as IAttrPanelPreferences;
};

const mergeCustomOrder = (currentOrder: Record<string, string[]>, patch?: Record<string, string[]>) => {
    const nextOrder: Record<string, string[]> = {};
    Object.keys(currentOrder || {}).forEach(key => {
        const list = currentOrder[key];
        nextOrder[key] = Array.isArray(list) ? list.slice() : [];
    });
    if (patch) {
        Object.keys(patch).forEach(key => {
            const list = patch[key];
            if (!list || list.length === 0) {
                delete nextOrder[key];
            } else {
                nextOrder[key] = list.slice();
            }
        });
    }
    return nextOrder;
};

const persistAttrPanelPreferences = (patch: Partial<IAttrPanelPreferences>) => {
    const current = ensureAttrPanelPreferences();
    const next: IAttrPanelPreferences = {
        ...current,
        ...patch,
        customOrder: mergeCustomOrder(current.customOrder, patch.customOrder)
    };
    window.siyuan.storage[Constants.LOCAL_ATTR_PANEL] = next;
    setStorageVal(Constants.LOCAL_ATTR_PANEL, next);
    return next;
};

const getAttrPanelSortLabel = (sortKey: TAttrPanelSortKey, languages: IObject) => {
    const nameAsc = languages?.fileNameASC || languages?.nameAsc || "名称 ↑";
    const nameDesc = languages?.fileNameDESC || languages?.nameDesc || "名称 ↓";
    const custom = languages?.customSort || languages?.custom || languages?.sort || "自定义排序";
    const createdAsc = languages?.createdASC || "创建时间 ↑";
    const createdDesc = languages?.createdDESC || "创建时间 ↓";
    const updatedAsc = languages?.modifiedASC || languages?.updatedASC || "更新时间 ↑";
    const updatedDesc = languages?.modifiedDESC || languages?.updatedDESC || "更新时间 ↓";
    const labelMap: Record<TAttrPanelSortKey, string> = {
        custom,
        nameAsc,
        nameDesc,
        createdAsc,
        createdDesc,
        updatedAsc,
        updatedDesc
    };
    return labelMap[sortKey] || custom;
};

export const setAttrPanelCustomOrder = (blockId: string, order: string[]) => {
    persistAttrPanelPreferences({customOrder: {[blockId]: Array.isArray(order) ? order.slice() : []}});
};

export const getAttrPanelCustomOrder = (blockId: string) => {
    const prefs = ensureAttrPanelPreferences();
    const order = prefs.customOrder?.[blockId];
    return Array.isArray(order) ? order.slice() : [];
};

interface IAttrPanelState {
    avTables: IAttrViewTableResponse[];
    viewBindings: IAttrPanelViewBinding[];
    dedupItems: IAttrPanelDedupItem[];
    aggregate?: IAttrPanelAggregateResponse;
}

const buildJumpTargetAttrs = (source?: IAttrPanelDedupSource): string => {
    if (!source) {
        return "disabled";
    }
    const base = "data-action=\"jumpField\"";
    if (source.targetType === "custom") {
        return `${base} data-target-type="custom" data-custom-name="${escapeAttr(source.customName || "")}"`;
    }
    return `${base} data-target-type="av" data-av-id="${escapeAttr(source.avId || "")}" data-key-id="${escapeAttr(source.keyId || "")}"`;
};

const renderAttrPanelDedupHtml = (fields: IAttrPanelDedupItem[], languages: IObject) => {
    if (!fields || fields.length === 0) {
        const emptyText = languages.empty || window.siyuan.languages.empty || window.siyuan.languages.notFound || "暂无属性";
        return `<div class="b3-label__text">${escapeHtml(emptyText)}</div>`;
    }
    return fields.map(field => {
        const firstSource = field.sources[0];
        const chips = field.sources.map(source => {
            const attrs = buildJumpTargetAttrs(source);
            return `<button class="b3-chip b3-chip--small attr-panel__dedup-chip" type="button" ${attrs}>${escapeHtml(source.label || languages.location || "定位")}</button>`;
        }).join("");
        const desc = field.desc ? `<div class="attr-panel__dedup-desc">${escapeHtml(field.desc)}</div>` : "";
        return `<div class="attr-panel__dedup-item" data-field-id="${escapeAttr(field.id)}">
    <div class="attr-panel__dedup-head">
        <button class="b3-button b3-button--text attr-panel__dedup-name" type="button" ${buildJumpTargetAttrs(firstSource)}>${escapeHtml(field.name)}</button>
    </div>
    ${desc}
    <div class="attr-panel__dedup-sources">${chips}</div>
</div>`;
    }).join("");
};

export const openWechatNotify = (nodeElement: Element) => {
    const id = nodeElement.getAttribute("data-node-id");
    const range = getEditorRange(nodeElement);
    const reminder = nodeElement.getAttribute(Constants.CUSTOM_REMINDER_WECHAT);
    let reminderFormat = "";
    if (reminder) {
        reminderFormat = dayjs(reminder).format("YYYY-MM-DD HH:mm");
    }
    const dialog = new Dialog({
        width: isMobile() ? "92vw" : "50vw",
        title: window.siyuan.languages.wechatReminder,
        content: `<div class="b3-dialog__content custom-attr">
    <div class="fn__flex">
        <span class="ft__on-surface fn__flex-center" style="text-align: right;white-space: nowrap;width: 100px">${window.siyuan.languages.notifyTime}</span>
        <div class="fn__space"></div>
        <input class="b3-text-field fn__flex-1" type="datetime-local" max="9999-12-31 23:59" value="${reminderFormat}">
    </div>
    <div class="b3-label__text" style="text-align: center">${window.siyuan.languages.wechatTip}</div>
</div>
<div class="b3-dialog__action">
    <button class="b3-button b3-button--cancel">${window.siyuan.languages.cancel}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.remove}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.confirm}</button>
</div>`,
        destroyCallback() {
            focusByRange(range);
        }
    });
    dialog.element.setAttribute("data-key", Constants.DIALOG_WECHATREMINDER);
    const btnsElement = dialog.element.querySelectorAll(".b3-button");
    btnsElement[0].addEventListener("click", () => {
        dialog.destroy();
    });
    btnsElement[1].addEventListener("click", () => {
        if (btnsElement[1].getAttribute("disabled")) {
            return;
        }
        btnsElement[1].setAttribute("disabled", "disabled");
        fetchPost("/api/block/setBlockReminder", {id, timed: "0"}, () => {
            nodeElement.removeAttribute(Constants.CUSTOM_REMINDER_WECHAT);
            dialog.destroy();
        });
    });
    btnsElement[2].addEventListener("click", () => {
        const date = dialog.element.querySelector("input").value;
        if (date) {
            if (new Date(date) <= new Date()) {
                showMessage(window.siyuan.languages.reminderTip);
                return;
            }
            if (btnsElement[2].getAttribute("disabled")) {
                return;
            }
            btnsElement[2].setAttribute("disabled", "disabled");
            const timed = dayjs(date).format("YYYYMMDDHHmmss");
            fetchPost("/api/block/setBlockReminder", {id, timed}, () => {
                nodeElement.setAttribute(Constants.CUSTOM_REMINDER_WECHAT, timed);
                dialog.destroy();
            });
        } else {
            showMessage(window.siyuan.languages.notEmpty);
        }
    });
};

export const openFileWechatNotify = (protyle: IProtyle) => {
    fetchPost("/api/block/getDocInfo", {
        id: protyle.block.rootID
    }, (response) => {
        const reminder = response.data.ial[Constants.CUSTOM_REMINDER_WECHAT];
        let reminderFormat = "";
        if (reminder) {
            reminderFormat = dayjs(reminder).format("YYYY-MM-DD HH:mm");
        }
        const dialog = new Dialog({
            width: isMobile() ? "92vw" : "50vw",
            title: window.siyuan.languages.wechatReminder,
            content: `<div class="b3-dialog__content custom-attr">
    <div class="fn__flex">
        <span class="ft__on-surface fn__flex-center" style="text-align: right;white-space: nowrap;width: 100px">${window.siyuan.languages.notifyTime}</span>
        <div class="fn__space"></div>
        <input class="b3-text-field fn__flex-1" type="datetime-local" max="9999-12-31 23:59" value="${reminderFormat}">
    </div>
    <div class="b3-label__text" style="text-align: center">${window.siyuan.languages.wechatTip}</div>
</div>
<div class="b3-dialog__action">
    <button class="b3-button b3-button--cancel">${window.siyuan.languages.cancel}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.remove}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.confirm}</button>
</div>`
        });
        dialog.element.setAttribute("data-key", Constants.DIALOG_WECHATREMINDER);
        const btnsElement = dialog.element.querySelectorAll(".b3-button");
        btnsElement[0].addEventListener("click", () => {
            dialog.destroy();
        });
        btnsElement[1].addEventListener("click", () => {
            fetchPost("/api/block/setBlockReminder", {id: protyle.block.rootID, timed: "0"}, () => {
                dialog.destroy();
            });
        });
        btnsElement[2].addEventListener("click", () => {
            const date = dialog.element.querySelector("input").value;
            if (date) {
                if (new Date(date) <= new Date()) {
                    showMessage(window.siyuan.languages.reminderTip);
                    return;
                }
                fetchPost("/api/block/setBlockReminder", {
                    id: protyle.block.rootID,
                    timed: dayjs(date).format("YYYYMMDDHHmmss")
                }, () => {
                    dialog.destroy();
                });
            } else {
                showMessage(window.siyuan.languages.notEmpty);
            }
        });
    });
};

export const openFileAttr = (attrs: IObject, focusName = "bookmark", protyle?: IProtyle) => {
    let customHTML = "";
    let notifyHTML = "";
    let customFieldEntries: IAttrPanelCustomAttrValue[] = [];
    const range = getSelection().rangeCount > 0 ? getSelection().getRangeAt(0) : null;
    let ghostProtyle: Protyle;
    if (!protyle) {
        getAllEditor().find(item => {
            if (attrs.id === item.protyle.block.rootID) {
                protyle = item.protyle;
                return true;
            }
        });
        if (!protyle) {
            ghostProtyle = new Protyle(window.siyuan.ws.app, document.createElement("div"), {
                blockId: attrs.id,
            });
        }
    }
    Object.keys(attrs).forEach(item => {
        if (Constants.CUSTOM_RIFF_DECKS === item || item.startsWith("custom-sy-")) {
            return;
        }
        if (item === Constants.CUSTOM_REMINDER_WECHAT) {
            notifyHTML = `<label class="b3-label b3-label--noborder">
    ${window.siyuan.languages.wechatReminder}
    <div class="fn__hr"></div>
    <input class="b3-text-field fn__block" type="datetime-local" max="9999-12-31 23:59" readonly data-name="${item}" value="${dayjs(attrs[item]).format("YYYY-MM-DD HH:mm")}">
</label>`;
        } else if (item.indexOf("custom") > -1) {
            customFieldEntries.push({name: item, label: item.replace("custom-", ""), value: attrs[item] || ""});
            customHTML += `<label class="b3-label b3-label--noborder">
     <div class="fn__flex">
        <span class="fn__flex-1">${item.replace("custom-", "")}</span>
        <span data-action="remove" class="block__icon block__icon--show"><svg><use xlink:href="#iconMin"></use></svg></span>
    </div>
    <div class="fn__hr"></div>
    <textarea style="resize: vertical;" spellcheck="false" class="b3-text-field fn__block" rows="1" data-name="${item}">${attrs[item]}</textarea>
</label>`;
        }
    });
    const languages = window.siyuan.languages;
    let attrPanelPrefs = ensureAttrPanelPreferences();
    const groupedLabel = languages.attrPanelGrouped || languages.groupedView || languages.category || "分类";
    const dedupLabel = languages.attrPanelDedup || languages.dedupView || "不重复";
    const sortLabel = languages.sort || "Sort";
    const addGaLabel = languages.addGlobalAttr || languages.globalAttr || "添加全局属性";
    const addDbLabel = languages.addToDatabase || "添加到数据库";
    const customLabel = languages.custom || "自定义";
    const showReadonlyLabel = languages.showReadonlyBuiltin || "显示只读内置属性";
    const panelTitle = languages.attr || languages.attrPanel || languages.globalAttr || window.siyuan.languages.attr;
    const dialog = new Dialog({
        width: isMobile() ? "92vw" : "50vw",
        containerClassName: "b3-dialog__container--theme",
        height: "80vh",
        title: panelTitle,
        content: `<div class="b3-dialog__content attr-panel" data-panel-mode="${attrPanelPrefs.mode}">
    <div class="attr-panel__toolbar">
        <button class="b3-button b3-button--text" data-action="openSortMenu">
            <svg><use xlink:href="#iconSort"></use></svg>
            <span data-role="sort-label">${sortLabel}</span>
        </button>
        <div class="layout-tab-bar layout-tab-bar--small">
            <div class="item item--small${attrPanelPrefs.mode === "grouped" ? " item--focus" : ""}" data-action="switchMode" data-mode="grouped">
                <span class="item__text">${groupedLabel}</span>
            </div>
            <div class="item item--small${attrPanelPrefs.mode === "dedup" ? " item--focus" : ""}" data-action="switchMode" data-mode="dedup">
                <span class="item__text">${dedupLabel}</span>
            </div>
        </div>
        <div class="fn__flex-1"></div>
    </div>
    <div class="attr-panel__body" data-role="panel-body">
        <div class="attr-panel__view attr-panel__view--grouped" data-role="grouped-view">
            <section class="attr-panel__section attr-panel__section--av" data-section="av"></section>
            <section class="attr-panel__section attr-panel__section--custom" data-section="custom">
                <header class="attr-panel__section-head">
                    <div class="attr-panel__section-title">${customLabel}</div>
                    <button data-action="addCustom" class="b3-button b3-button--outline attr-panel__add-custom">
                        <svg><use xlink:href="#iconAdd"></use></svg>${languages.addAttr}
                    </button>
                </header>
                <div class="attr-panel__custom-list" data-role="custom-list">
                    <div class="attr-panel__custom-items" data-role="custom-items">
                        ${customHTML}
                    </div>
                    ${notifyHTML ? `<div class="attr-panel__custom-notify">${notifyHTML}</div>` : ""}
                </div>
            </section>
        </div>
        <div class="attr-panel__view attr-panel__view--dedup fn__none" data-role="dedup-view"></div>
    </div>
</div>
<div class="b3-dialog__action attr-panel__footer">
    <button data-action="addToDatabase" class="b3-button b3-button--text">
        <svg><use xlink:href="#iconDatabase"></use></svg>${addDbLabel}
    </button>
    <button data-action="addGlobalAttr" class="b3-button b3-button--text">
        <svg><use xlink:href="#iconAttr"></use></svg>${addGaLabel}
    </button>
    <div class="fn__flex-1"></div>
    <label class="b3-label b3-label--noborder attr-panel__readonly-toggle">
        <input type="checkbox" class="b3-switch" data-action="toggleReadonly" ${attrPanelPrefs.showReadonlyBuiltin ? "checked" : ""}>
        <span>${showReadonlyLabel}</span>
    </label>
</div>`,
        destroyCallback() {
            focusByRange(range);
            if (protyle) {
                hideElements(["select"], protyle);
            } else {
                ghostProtyle?.destroy();
            }
        }
    });

    const groupedViewElement = dialog.element.querySelector<HTMLElement>('[data-role="grouped-view"]');
    const dedupViewElement = dialog.element.querySelector<HTMLElement>('[data-role="dedup-view"]');
    const avSectionElement = dialog.element.querySelector('[data-section="av"]') as HTMLElement;
    const customItemsElement = dialog.element.querySelector<HTMLElement>('[data-role="custom-items"]');

    const updateViewModeVisibility = (mode: TAttrPanelMode) => {
        groupedViewElement?.classList.toggle("fn__none", mode === "dedup");
        dedupViewElement?.classList.toggle("fn__none", mode !== "dedup");
    };
    updateViewModeVisibility(attrPanelPrefs.mode);

    const attrPanelState: IAttrPanelState = {
        avTables: [],
        viewBindings: [],
        dedupItems: []
    };
    const renderDedupSummary = (tables?: IAttrViewTableResponse[]) => {
        if (tables) {
            attrPanelState.avTables = tables;
        }
        attrPanelState.viewBindings = buildAttrPanelViewBindingsFromTables(attrPanelState.avTables, languages);
        attrPanelState.dedupItems = buildAttrPanelDedupItems(attrPanelState.viewBindings, customFieldEntries, languages);
        attrPanelState.aggregate = buildAttrPanelAggregateResponse(attrs.id, attrPanelState.avTables, customFieldEntries, languages, {
            prebuiltViewBindings: attrPanelState.viewBindings,
            prebuiltDedupItems: attrPanelState.dedupItems
        });
        if (dedupViewElement) {
            dedupViewElement.innerHTML = renderAttrPanelDedupHtml(attrPanelState.dedupItems, languages);
        }
    };
    renderDedupSummary();
    dialog.element.setAttribute("data-key", Constants.DIALOG_ATTR);
    dialog.element.setAttribute("data-sort-mode", attrPanelPrefs.sort);
    dialog.element.setAttribute("data-show-readonly", attrPanelPrefs.showReadonlyBuiltin ? "1" : "0");
    const sortLabelSpan = dialog.element.querySelector<HTMLSpanElement>('[data-role="sort-label"]');
    const updateSortButtonLabel = () => {
        if (sortLabelSpan) {
            sortLabelSpan.textContent = `${sortLabel} · ${getAttrPanelSortLabel(attrPanelPrefs.sort, languages)}`;
        }
        dialog.element.setAttribute("data-sort-mode", attrPanelPrefs.sort);
    };
    updateSortButtonLabel();
    const showSortMenu = (trigger: HTMLElement) => {
        const menu = new PopupMenu(Constants.MENU_ATTR_PANEL_SORT);
        if (menu.isOpen) {
            return;
        }
        menu.element.classList.add("b3-menu--list");
        ATTR_PANEL_SORT_GROUPS.forEach((group, index) => {
            if (index !== 0) {
                menu.addSeparator();
            }
            group.forEach(sortKey => {
                menu.addItem({
                    icon: attrPanelPrefs.sort === sortKey ? "iconSelect" : undefined,
                    label: getAttrPanelSortLabel(sortKey, languages),
                    click: () => {
                        if (attrPanelPrefs.sort === sortKey) {
                            return;
                        }
                        attrPanelPrefs = persistAttrPanelPreferences({sort: sortKey});
                        updateSortButtonLabel();
                    }
                });
            });
        });
        const rect = trigger.getBoundingClientRect();
        menu.open({
            x: rect.left,
            y: rect.bottom + 4
        });
    };
    const readonlySwitchElement = dialog.element.querySelector<HTMLInputElement>("[data-action='toggleReadonly']");
    if (readonlySwitchElement) {
        readonlySwitchElement.checked = !!attrPanelPrefs.showReadonlyBuiltin;
    }

    const protyleCtx = protyle || ghostProtyle?.protyle;
    const applyReadonlyFilter = (showReadonly: boolean) => {
        avSectionElement?.querySelectorAll<HTMLElement>(`[data-av-id="${BUILTIN_ATTR_VIEW_ID}"] .av__row`).forEach(row => {
            const writable = row.querySelector("[data-ga-writable]")?.getAttribute("data-ga-writable") === "true";
            row.classList.toggle("fn__none", !showReadonly && !writable);
        });
    };
    const focusAvByName = (name: string) => {
        if (!name || !avSectionElement) {
            return;
        }
        const encoded = escapeAttr(name);
        const targetCell = Array.from(avSectionElement.querySelectorAll<HTMLElement>("[data-col-name]")).find(cell => cell.getAttribute("data-col-name") === encoded);
        if (targetCell) {
            const input = targetCell.querySelector<HTMLInputElement | HTMLTextAreaElement>("input,textarea");
            if (input) {
                input.focus();
            } else {
                (targetCell as HTMLElement).focus?.();
            }
            targetCell.scrollIntoView({block: "center"});
        }
    };
    if (protyleCtx && avSectionElement) {
        renderAVAttribute(avSectionElement, attrs.id, protyleCtx, (_element, tables) => {
            const readonlySwitch = dialog.element.querySelector<HTMLInputElement>("[data-action='toggleReadonly']");
            applyReadonlyFilter(!!readonlySwitch?.checked);
            if (focusName && focusName !== "custom" && focusName !== "av") {
                focusAvByName(focusName);
            }
            renderDedupSummary(tables);
        });
    }

    if (focusName === "custom") {
        dialog.element.querySelector<HTMLInputElement | HTMLTextAreaElement>('[data-section="custom"] textarea, [data-section="custom"] input')?.focus();
    } else if (focusName === "av") {
        avSectionElement?.querySelector<HTMLElement>(`[data-av-id]:not([data-av-id="${BUILTIN_ATTR_VIEW_ID}"])`)?.scrollIntoView({block: "start"});
    }

    const handleAddToDatabase = () => {
        if (!protyleCtx) {
            return;
        }
        let selectionRange = range;
        if (!selectionRange) {
            const selection = getSelection();
            if (selection && selection.rangeCount > 0) {
                selectionRange = selection.getRangeAt(0);
            }
        }
        if (!selectionRange) {
            const fallbackRange = document.createRange();
            fallbackRange.selectNodeContents(protyleCtx?.wysiwyg?.element || document.body);
            selectionRange = fallbackRange;
        }
        addEditorToDatabase(protyleCtx, selectionRange);
    };

    dialog.element.addEventListener("click", (event) => {
        const actionEl = (event.target as HTMLElement).closest("[data-action]") as HTMLElement;
        if (!actionEl) {
            return;
        }
        const action = actionEl.dataset.action;
        switch (action) {
        case "openSortMenu":
            showSortMenu(actionEl);
            event.preventDefault();
            break;
        case "remove":
                fetchPost("/api/attr/setBlockAttrs", {
                    id: attrs.id,
                attrs: {["custom-" + actionEl.previousElementSibling.textContent]: ""}
                });
            actionEl.parentElement.parentElement.remove();
            {
                const removedLabel = actionEl.previousElementSibling?.textContent?.trim();
                if (removedLabel) {
                    const removedKey = `custom-${removedLabel}`;
                    customFieldEntries = customFieldEntries.filter(entry => entry.name !== removedKey);
                    renderDedupSummary();
                }
            }
            event.preventDefault();
            break;
        case "addCustom": {
            if (!customItemsElement) {
                event.preventDefault();
                break;
            }
                const addDialog = new Dialog({
                    title: window.siyuan.languages.attrName,
                    content: `<div class="b3-dialog__content"><input spellcheck="false" class="b3-text-field fn__block" value=""></div>
<div class="b3-dialog__action">
    <button class="b3-button b3-button--cancel">${window.siyuan.languages.cancel}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.confirm}</button>
</div>`,
                    width: isMobile() ? "92vw" : "520px",
                });
                addDialog.element.setAttribute("data-key", Constants.DIALOG_SETCUSTOMATTR);
                const inputElement = addDialog.element.querySelector("input") as HTMLInputElement;
                const btnsElement = addDialog.element.querySelectorAll(".b3-button");
                addDialog.bindInput(inputElement, () => {
                    (btnsElement[1] as HTMLButtonElement).click();
                });
                inputElement.focus();
                inputElement.select();
                btnsElement[0].addEventListener("click", () => {
                    addDialog.destroy();
                });
                btnsElement[1].addEventListener("click", () => {
                    const normalizedName = inputElement.value.trim();
                    inputElement.value = normalizedName;
                    if (!isValidAttrName(normalizedName)) {
                        showMessage(window.siyuan.languages.attrName + " <b>" + escapeHtml(normalizedName) + "</b> " + window.siyuan.languages.invalid);
                        return false;
                    }
                    customItemsElement.insertAdjacentHTML("beforeend", `<div class="b3-label b3-label--noborder">
    <div class="fn__flex">
        <span class="fn__flex-1">${normalizedName}</span>
        <span data-action="remove" class="block__icon block__icon--show"><svg><use xlink:href="#iconMin"></use></svg></span>
    </div>
    <div class="fn__hr"></div>
    <textarea style="resize: vertical" spellcheck="false" data-name="custom-${normalizedName}" class="b3-text-field fn__block" rows="1" placeholder="${window.siyuan.languages.attrValue1}"></textarea>
</div>`);
                    const newFieldElement = customItemsElement.lastElementChild as HTMLElement;
                    const valueElement = newFieldElement?.querySelector(".b3-text-field") as HTMLTextAreaElement;
                    if (valueElement) {
                        valueElement.focus();
                        bindAttrInput(valueElement, attrs.id);
                    }
                    const attrKey = `custom-${normalizedName}`;
                    if (!customFieldEntries.find(entry => entry.name === attrKey)) {
                        customFieldEntries.push({name: attrKey, label: normalizedName, value: ""});
                        renderDedupSummary();
                    }
                    addDialog.destroy();
                });
                event.preventDefault();
                break;
            }
        case "addToDatabase":
            handleAddToDatabase();
            event.preventDefault();
            break;
        case "addGlobalAttr":
            showMessage(window.siyuan.languages.comingSoon || "Adding global attributes from here is under construction");
            event.preventDefault();
            break;
        case "toggleReadonly":
            {
                const showReadonly = (actionEl as HTMLInputElement).checked;
                applyReadonlyFilter(showReadonly);
                attrPanelPrefs = persistAttrPanelPreferences({showReadonlyBuiltin: showReadonly});
                dialog.element.setAttribute("data-show-readonly", showReadonly ? "1" : "0");
            }
            break;
        case "switchMode":
            {
                const nextMode = (actionEl.dataset.mode as TAttrPanelMode) || "grouped";
                if (nextMode !== attrPanelPrefs.mode) {
                    dialog.element.setAttribute("data-panel-mode", nextMode);
                    attrPanelPrefs = persistAttrPanelPreferences({mode: nextMode});
                }
                const modeBar = actionEl.parentElement;
                modeBar?.querySelectorAll(".item").forEach(btn => btn.classList.remove("item--focus"));
                actionEl.classList.add("item--focus");
                updateViewModeVisibility(nextMode);
            }
            break;
        case "jumpField":
            {
                const targetType = (actionEl.dataset.targetType as TAttrPanelFieldTarget) || "av";
                let targetElement: HTMLElement | null = null;
                if (targetType === "custom") {
                    const customName = actionEl.dataset.customName;
                    if (customName) {
                        targetElement = dialog.element.querySelector(`[data-section="custom"] [data-name="${customName}"]`) as HTMLElement;
                    }
                } else {
                    const avId = actionEl.dataset.avId;
                    const keyId = actionEl.dataset.keyId;
                    if (avId && keyId && avSectionElement) {
                        targetElement = avSectionElement.querySelector(`[data-av-id="${avId}"] .av__row[data-col-id="${keyId}"]`) as HTMLElement;
                    }
                }
                if (targetElement) {
                    targetElement.scrollIntoView({block: "center"});
                    if (targetElement instanceof HTMLInputElement || targetElement instanceof HTMLTextAreaElement) {
                        targetElement.focus();
                    } else {
                        targetElement.querySelector<HTMLInputElement | HTMLTextAreaElement>("input,textarea")?.focus();
                    }
                }
                event.preventDefault();
            }
            break;
        }
    });
    dialog.element.querySelectorAll<HTMLTextAreaElement>('[data-section="custom"] textarea[data-name]').forEach(item => {
        bindAttrInput(item, attrs.id);
    });
};

export const openAttr = (nodeElement: Element, focusName = "bookmark", protyle: IProtyle) => {
    if (nodeElement.getAttribute("data-type") === "NodeThematicBreak") {
        return;
    }
    const id = nodeElement.getAttribute("data-node-id");
    fetchPost("/api/attr/getBlockAttrs", {id}, (response) => {
        openFileAttr(response.data, focusName, protyle);
    });
};

export const copySubMenu = (ids: string[], accelerator = true, focusElement?: Element, stdMarkdownId?: string) => {
    const menuItems = [{
        id: "copyBlockRef",
        iconHTML: "",
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyBlockRef.custom : undefined,
        label: window.siyuan.languages.copyBlockRef,
        click: () => {
            copyTextByType(ids, "ref");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }, {
        id: "copyBlockEmbed",
        iconHTML: "",
        label: window.siyuan.languages.copyBlockEmbed,
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyBlockEmbed.custom : undefined,
        click: () => {
            copyTextByType(ids, "blockEmbed");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }, {
        id: "copyProtocol",
        iconHTML: "",
        label: window.siyuan.languages.copyProtocol,
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyProtocol.custom : undefined,
        click: () => {
            copyTextByType(ids, "protocol");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }, {
        id: "copyProtocolInMd",
        iconHTML: "",
        label: window.siyuan.languages.copyProtocolInMd,
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyProtocolInMd.custom : undefined,
        click: () => {
            copyTextByType(ids, "protocolMd");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }, {
        id: "copyHPath",
        iconHTML: "",
        label: window.siyuan.languages.copyHPath,
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyHPath.custom : undefined,
        click: () => {
            copyTextByType(ids, "hPath");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }, {
        id: "copyID",
        iconHTML: "",
        label: window.siyuan.languages.copyID,
        accelerator: accelerator ? window.siyuan.config.keymap.editor.general.copyID.custom : undefined,
        click: () => {
            copyTextByType(ids, "id");
            if (focusElement) {
                focusBlock(focusElement);
            }
        }
    }];

    if (stdMarkdownId) {
        menuItems.push({
            id: "copyMarkdown",
            iconHTML: "",
            label: window.siyuan.languages.copyMarkdown,
            accelerator: undefined,
            click: async () => {
                const response = await fetchSyncPost("/api/export/exportMdContent", {
                    id: stdMarkdownId,
                    refMode: 3,
                    embedMode: 1,
                    yfm: false,
                    fillCSSVar: false,
                    adjustHeadingLevel: false
                });
                const text = response.data.content;
                writeText(text);
                if (focusElement) {
                    focusBlock(focusElement);
                }
            }
        });
    }

    return menuItems;
};

export const exportMd = (id: string) => {
    if (window.siyuan.isPublish) {
        return;
    }
    return new MenuItem({
        id: "export",
        label: window.siyuan.languages.export,
        type: "submenu",
        icon: "iconUpload",
        submenu: [{
            id: "exportTemplate",
            label: window.siyuan.languages.template,
            iconClass: "ft__error",
            icon: "iconMarkdown",
            click: async () => {
                const result = await fetchSyncPost("/api/block/getRefText", {id: id});

                const dialog = new Dialog({
                    title: window.siyuan.languages.fileName,
                    content: `<div class="b3-dialog__content"><input class="b3-text-field fn__block" value=""></div>
<div class="b3-dialog__action">
    <button class="b3-button b3-button--cancel">${window.siyuan.languages.cancel}</button><div class="fn__space"></div>
    <button class="b3-button b3-button--text">${window.siyuan.languages.confirm}</button>
</div>`,
                    width: isMobile() ? "92vw" : "520px",
                });
                dialog.element.setAttribute("data-key", Constants.DIALOG_EXPORTTEMPLATE);
                const inputElement = dialog.element.querySelector("input") as HTMLInputElement;
                const btnsElement = dialog.element.querySelectorAll(".b3-button");
                dialog.bindInput(inputElement, () => {
                    (btnsElement[1] as HTMLButtonElement).click();
                });
                let name = replaceFileName(result.data);
                const maxNameLen = 32;
                if (name.length > maxNameLen) {
                    name = name.substring(0, maxNameLen);
                }
                inputElement.value = name;
                inputElement.focus();
                inputElement.select();
                btnsElement[0].addEventListener("click", () => {
                    dialog.destroy();
                });
                btnsElement[1].addEventListener("click", () => {
                    if (inputElement.value.trim() === "") {
                        inputElement.value = window.siyuan.languages.untitled;
                    } else {
                        inputElement.value = replaceFileName(inputElement.value);
                    }

                    if (name.length > maxNameLen) {
                        name = name.substring(0, maxNameLen);
                    }

                    fetchPost("/api/template/docSaveAsTemplate", {
                        id,
                        name: inputElement.value,
                        overwrite: false
                    }, response => {
                        if (response.code === 1) {
                            // 重名
                            confirmDialog(window.siyuan.languages.export, window.siyuan.languages.exportTplTip, () => {
                                fetchPost("/api/template/docSaveAsTemplate", {
                                    id,
                                    name: inputElement.value,
                                    overwrite: true
                                }, resp => {
                                    if (resp.code === 0) {
                                        showMessage(window.siyuan.languages.exportTplSucc);
                                    }
                                });
                            });
                            return;
                        }
                        showMessage(window.siyuan.languages.exportTplSucc);
                    });
                    dialog.destroy();
                });
            }
        }, {
            id: "exportSiYuanZip",
            label: "SiYuan .sy.zip",
            icon: "iconSiYuan",
            click: () => {
                const msgId = showMessage(window.siyuan.languages.exporting, -1);
                fetchPost("/api/export/exportSY", {
                    id,
                }, response => {
                    hideMessage(msgId);
                    openByMobile(response.data.zip);
                });
            }
        }, {
            id: "exportMarkdown",
            label: "Markdown .zip",
            icon: "iconMarkdown",
            click: () => {
                const msgId = showMessage(window.siyuan.languages.exporting, -1);
                fetchPost("/api/export/exportMd", {
                    id,
                }, response => {
                    hideMessage(msgId);
                    openByMobile(response.data.zip);
                });
            }
        }, {
            id: "exportImage",
            label: window.siyuan.languages.image,
            icon: "iconImage",
            click: () => {
                exportImage(id);
            }
        },
            /// #if !BROWSER
            {
                id: "exportPDF",
                label: "PDF",
                icon: "iconPDF",
                click: () => {
                    saveExport({type: "pdf", id});
                }
            }, {
                id: "exportHTML_SiYuan",
                label: "HTML (SiYuan)",
                iconClass: "ft__error",
                icon: "iconHTML5",
                click: () => {
                    saveExport({type: "html", id});
                }
            }, {
                id: "exportHTML_Markdown",
                label: "HTML (Markdown)",
                icon: "iconHTML5",
                click: () => {
                    saveExport({type: "htmlmd", id});
                }
            }, {
                id: "exportWord",
                label: "Word .docx",
                icon: "iconExact",
                click: () => {
                    saveExport({type: "word", id});
                }
            }, {
                id: "exportMore",
                label: window.siyuan.languages.more,
                icon: "iconMore",
                type: "submenu",
                submenu: [{
                    id: "exportReStructuredText",
                    label: "reStructuredText",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportReStructuredText", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportAsciiDoc",
                    label: "AsciiDoc",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportAsciiDoc", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportTextile",
                    label: "Textile",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportTextile", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportOPML",
                    label: "OPML",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportOPML", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportOrgMode",
                    label: "Org-Mode",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportOrgMode", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportMediaWiki",
                    label: "MediaWiki",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportMediaWiki", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportODT",
                    label: "ODT",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportODT", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportRTF",
                    label: "RTF",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportRTF", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                }, {
                    id: "exportEPUB",
                    label: "EPUB",
                    click: () => {
                        const msgId = showMessage(window.siyuan.languages.exporting, -1);
                        fetchPost("/api/export/exportEPUB", {
                            id,
                        }, response => {
                            hideMessage(msgId);
                            openByMobile(response.data.zip);
                        });
                    }
                },
                ]
            },
            /// #else
            {
                id: "exportPDF",
                label: window.siyuan.languages.print,
                icon: "iconPDF",
                ignore: !isInAndroid() && !isInHarmony(),
                click: () => {
                    const msgId = showMessage(window.siyuan.languages.exporting);
                    const localData = window.siyuan.storage[Constants.LOCAL_EXPORTPDF];
                    fetchPost("/api/export/exportPreviewHTML", {
                        id,
                        keepFold: localData.keepFold,
                        merge: localData.mergeSubdocs,
                    }, async response => {
                        const servePath = window.location.protocol + "//" + window.location.host + "/";
                        const html = await onExport(response, undefined, servePath, {type: "pdf", id});
                        if (isInAndroid()) {
                            window.JSAndroid.print(html);
                        } else if (isInHarmony()) {
                            window.JSHarmony.print(html);
                        }

                        setTimeout(() => {
                            hideMessage(msgId);
                        }, 3000);
                    });
                }
            }, {
                id: "exportHTML_SiYuan",
                label: "HTML (SiYuan)",
                iconClass: "ft__error",
                icon: "iconHTML5",
                click: () => {
                    saveExport({type: "html", id});
                }
            }, {
                id: "exportHTML_Markdown",
                label: "HTML (Markdown)",
                icon: "iconHTML5",
                click: () => {
                    saveExport({type: "htmlmd", id});
                }
            },
            /// #endif
        ]
    }).element;
};

export const openMenu = (app: App, src: string, onlyMenu: boolean, showAccelerator: boolean) => {
    const submenu = [];
    /// #if MOBILE
    submenu.push({
        id: isInAndroid() ? "useDefault" : "useBrowserView",
        label: isInAndroid() ? window.siyuan.languages.useDefault : window.siyuan.languages.useBrowserView,
        accelerator: showAccelerator ? window.siyuan.languages.click : "",
        click: () => {
            openByMobile(src);
        }
    });
    /// #else
    if (isLocalPath(src)) {
        if (Constants.SIYUAN_ASSETS_EXTS.includes(pathPosix().extname(src).split("?")[0]) &&
            (!src.endsWith(".pdf") ||
                (src.endsWith(".pdf") && !src.startsWith("file://")))
        ) {
            submenu.push({
                id: "insertRight",
                icon: "iconLayoutRight",
                label: window.siyuan.languages.insertRight,
                accelerator: showAccelerator ? window.siyuan.languages.click : "",
                click() {
                    openAsset(app, src.trim(), parseInt(getSearch("page", src)), "right");
                }
            });
            submenu.push({
                id: "openBy",
                label: window.siyuan.languages.openBy,
                icon: "iconOpen",
                accelerator: showAccelerator ? "⌥" + window.siyuan.languages.click : "",
                click() {
                    openAsset(app, src.trim(), parseInt(getSearch("page", src)));
                }
            });
            /// #if !BROWSER
            submenu.push({
                id: "openByNewWindow",
                label: window.siyuan.languages.openByNewWindow,
                icon: "iconOpenWindow",
                click() {
                    openAssetNewWindow(src.trim());
                }
            });
            submenu.push({
                id: "showInFolder",
                icon: "iconFolder",
                label: window.siyuan.languages.showInFolder,
                accelerator: showAccelerator ? "⌘" + window.siyuan.languages.click : "",
                click: () => {
                    openBy(src, "folder");
                }
            });
            submenu.push({
                id: "useDefault",
                label: window.siyuan.languages.useDefault,
                accelerator: showAccelerator ? "⇧" + window.siyuan.languages.click : "",
                click() {
                    openBy(src, "app");
                }
            });
            /// #endif
        } else {
            /// #if !BROWSER
            submenu.push({
                id: "useDefault",
                label: window.siyuan.languages.useDefault,
                accelerator: showAccelerator ? window.siyuan.languages.click : "",
                click() {
                    openBy(src, "app");
                }
            });
            submenu.push({
                id: "showInFolder",
                icon: "iconFolder",
                label: window.siyuan.languages.showInFolder,
                accelerator: showAccelerator ? "⌘" + window.siyuan.languages.click : "",
                click: () => {
                    openBy(src, "folder");
                }
            });
            /// #else
            submenu.push({
                id: isInAndroid() || isInHarmony() ? "useDefault" : "useBrowserView",
                label: isInAndroid() || isInHarmony() ? window.siyuan.languages.useDefault : window.siyuan.languages.useBrowserView,
                accelerator: showAccelerator ? window.siyuan.languages.click : "",
                click: () => {
                    openByMobile(src);
                }
            });
            /// #endif
        }
    } else if (src) {
        if (0 > src.indexOf(":")) {
            // 使用 : 判断，不使用 :// 判断 Open external application protocol invalid https://github.com/siyuan-note/siyuan/issues/10075
            // Support click to open hyperlinks like `www.foo.com` https://github.com/siyuan-note/siyuan/issues/9986
            src = `https://${src}`;
        }
        /// #if !BROWSER
        submenu.push({
            id: "useDefault",
            label: window.siyuan.languages.useDefault,
            accelerator: showAccelerator ? window.siyuan.languages.click : "",
            click: () => {
                shell.openExternal(src).catch((e) => {
                    showMessage(e);
                });
            }
        });
        /// #else
        submenu.push({
            id: isInAndroid() || isInHarmony() ? "useDefault" : "useBrowserView",
            label: isInAndroid() || isInHarmony() ? window.siyuan.languages.useDefault : window.siyuan.languages.useBrowserView,
            accelerator: showAccelerator ? window.siyuan.languages.click : "",
            click: () => {
                openByMobile(src);
            }
        });
        /// #endif
    }
    /// #endif
    if (onlyMenu) {
        return submenu;
    }
    window.siyuan.menus.menu.append(new MenuItem({
        id: "openBy",
        label: window.siyuan.languages.openBy,
        icon: "iconOpen",
        submenu
    }).element);
};

export const renameMenu = (options: {
    path: string
    notebookId: string
    name: string,
    type: "notebook" | "file"
}) => {
    return new MenuItem({
        id: "rename",
        accelerator: window.siyuan.config.keymap.editor.general.rename.custom,
        icon: "iconEdit",
        label: window.siyuan.languages.rename,
        click: () => {
            rename(options);
        }
    }).element;
};

export const movePathToMenu = (paths: string[]) => {
    return new MenuItem({
        id: "move",
        label: window.siyuan.languages.move,
        icon: "iconMove",
        accelerator: window.siyuan.config.keymap.general.move.custom,
        click() {
            movePathTo((toPath, toNotebook) => {
                moveToPath(paths, toNotebook[0], toPath[0]);
            }, paths);
        }
    }).element;
};
