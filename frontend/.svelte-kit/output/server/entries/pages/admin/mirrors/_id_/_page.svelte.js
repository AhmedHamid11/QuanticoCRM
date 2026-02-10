import { a2 as ssr_context, W as store_get, _ as unsubscribe_stores } from "../../../../../chunks/index.js";
import "clsx";
import { p as page } from "../../../../../chunks/stores.js";
import "../../../../../chunks/auth.svelte.js";
import { e as escape_html } from "../../../../../chunks/attributes.js";
function onDestroy(fn) {
  /** @type {SSRContext} */
  ssr_context.r.on_destroy(fn);
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).params.id;
    let editedSourceFields = [];
    editedSourceFields.filter((f) => f.mapField !== null && f.mapField !== "").length;
    onDestroy(() => {
    });
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div class="flex items-center gap-4"><div><div class="flex items-center gap-3"><h1 class="text-2xl font-bold text-gray-900">${escape_html("Loading...")}</h1> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> <p class="mt-1 text-sm text-gray-500">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></p></div></div> <a href="/admin/mirrors" class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50">Back to Mirrors</a></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-white shadow rounded-lg p-6"><div class="animate-pulse space-y-4"><div class="h-4 bg-gray-200 rounded w-1/4"></div> <div class="h-10 bg-gray-200 rounded w-1/2"></div></div></div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
