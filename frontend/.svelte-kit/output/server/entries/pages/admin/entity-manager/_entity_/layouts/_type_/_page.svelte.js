import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../../../chunks/index.js";
import { p as page } from "../../../../../../../chunks/stores.js";
import "../../../../../../../chunks/auth.svelte.js";
import { a as attr, e as escape_html } from "../../../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    let layoutType = store_get($$store_subs ??= {}, "$page", page).params.type;
    let isListLayout = layoutType === "list";
    let fields = [];
    let listColumns = [];
    let usedFieldNames = () => {
      return /* @__PURE__ */ new Set();
    };
    const layoutTypeInfo = {
      list: {
        name: "List",
        description: "Configure which columns appear in the list/table view"
      },
      detail: {
        name: "Detail",
        description: "Configure field sections for the record detail page"
      },
      detailSmall: {
        name: "Quick View",
        description: "Configure the compact view shown in modals and popovers"
      },
      filters: {
        name: "Filters",
        description: "Configure which fields are available in the search/filter panel"
      },
      massUpdate: {
        name: "Mass Update",
        description: "Configure which fields are available for bulk editing"
      }
    };
    fields.filter((f) => !listColumns.includes(f.name));
    fields.filter((f) => !usedFieldNames().has(f.name));
    $$renderer2.push(`<div class="space-y-6"><div><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="hover:text-gray-700">${escape_html(entityName)}</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}/layouts`)} class="hover:text-gray-700">Layouts</a> <span class="mx-2">/</span> <span class="text-gray-900">${escape_html(layoutTypeInfo[layoutType]?.name || layoutType)}</span></nav> <div class="flex items-center justify-between"><div><h1 class="text-2xl font-bold text-gray-900">${escape_html(entityName)} - ${escape_html(layoutTypeInfo[layoutType]?.name || layoutType)} Layout</h1> <p class="text-sm text-gray-500 mt-1">${escape_html(layoutTypeInfo[layoutType]?.description || "")}</p></div> <div class="flex gap-2">`);
    if (!isListLayout) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<button type="button"${attr("disabled", true, true)} class="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed">Add Section</button>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <button type="button"${attr("disabled", isListLayout ? false : true, true)} class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90 disabled:opacity-50">${escape_html("Save Layout")}</button></div></div></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
