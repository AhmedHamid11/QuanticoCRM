import { W as store_get, _ as unsubscribe_stores } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { e as escape_html } from "../../../../../chunks/attributes.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entityName = store_get($$store_subs ??= {}, "$page", page).params.entity;
    $$renderer2.push(`<div class="space-y-6"><div><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/entity-manager" class="hover:text-gray-700">Entity Manager</a> <span class="mx-2">/</span> <span class="text-gray-900">${escape_html(entityName)}</span></nav> <div class="flex items-center gap-4">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div><h1 class="text-2xl font-bold text-gray-900">${escape_html(entityName)}</h1> <p class="text-sm text-gray-500">${escape_html("")}</p></div></div></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
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
