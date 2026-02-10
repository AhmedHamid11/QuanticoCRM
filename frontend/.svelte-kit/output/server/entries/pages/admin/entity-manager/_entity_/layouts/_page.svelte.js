import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../../chunks/index.js";
import { p as page } from "../../../../../../chunks/stores.js";
import "../../../../../../chunks/auth.svelte.js";
import { a as attr, e as escape_html } from "../../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    $$renderer2.push(`<div class="space-y-6"><div><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="hover:text-gray-700">${escape_html(entityName)}</a> <span class="mx-2">/</span> <span class="text-gray-900">Layouts</span></nav> <h1 class="text-2xl font-bold text-gray-900">${escape_html(entityName)} - Layouts</h1></div> `);
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
