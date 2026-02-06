import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../../chunks/index.js";
import { p as page } from "../../../../../../chunks/stores.js";
import "../../../../../../chunks/auth.svelte.js";
import { a as attr, e as escape_html } from "../../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    let fields = [];
    let searchQuery = "";
    (() => {
      if (!searchQuery.trim()) return fields;
      const query = searchQuery.toLowerCase();
      return fields.filter((f) => f.label.toLowerCase().includes(query) || f.name.toLowerCase().includes(query) || f.type.toLowerCase().includes(query));
    })();
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="hover:text-gray-700">${escape_html(entityName)}</a> <span class="mx-2">/</span> <span class="text-gray-900">Fields</span></nav> <h1 class="text-2xl font-bold text-gray-900">${escape_html(entityName)} - Fields</h1></div> <button class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90">+ Add Field</button></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading fields...</div>`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
