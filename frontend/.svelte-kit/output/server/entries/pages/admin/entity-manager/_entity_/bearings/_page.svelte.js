import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../../chunks/index.js";
import { p as page } from "../../../../../../chunks/stores.js";
import { a as attr, e as escape_html } from "../../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    let picklistFields = [];
    $$renderer2.push(`<div class="space-y-6"><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="hover:text-gray-700">${escape_html(entityName)}</a> <span class="mx-2">/</span> <span class="text-gray-900">Bearings</span></nav> <div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Bearings</h1> <p class="text-sm text-gray-500 mt-1">Visual stage progress indicators for ${escape_html(entityName)} records</p></div> <button${attr("disabled", picklistFields.length === 0, true)} class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed">+ New Bearing</button></div> `);
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
