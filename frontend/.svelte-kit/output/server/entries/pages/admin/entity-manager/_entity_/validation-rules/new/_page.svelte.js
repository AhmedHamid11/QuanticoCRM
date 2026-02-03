import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../../../chunks/index.js";
import { p as page } from "../../../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../../../chunks/exports.js";
import "../../../../../../../chunks/utils.js";
import { a as attr, e as escape_html } from "../../../../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../../../../chunks/state.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    $$renderer2.push(`<div class="space-y-6 max-w-4xl"><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="hover:text-gray-700">${escape_html(entityName)}</a> <span class="mx-2">/</span> <a${attr("href", `/admin/entity-manager/${stringify(entityName)}/validation-rules`)} class="hover:text-gray-700">Validation Rules</a> <span class="mx-2">/</span> <span class="text-gray-900">New Rule</span></nav> <h1 class="text-2xl font-bold text-gray-900">Create Validation Rule</h1> `);
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
