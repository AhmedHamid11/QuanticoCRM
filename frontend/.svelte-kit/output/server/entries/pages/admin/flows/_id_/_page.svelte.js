import { W as store_get, _ as unsubscribe_stores } from "../../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { e as escape_html } from "../../../../../chunks/attributes.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import { p as page } from "../../../../../chunks/stores.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).params.id;
    let version = 1;
    $$renderer2.push(`<div class="max-w-6xl mx-auto space-y-6"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Edit Screen Flow</h1> <p class="text-sm text-gray-500 mt-1">Version ${escape_html(version)}</p></div> <a href="/admin/flows" class="text-sm text-gray-600 hover:text-gray-900">← Back to Flows</a></div> `);
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
