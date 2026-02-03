import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { a as attr } from "../../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import { F as FormSkeleton } from "../../../../../chunks/FormSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let contactId = store_get($$store_subs ??= {}, "$page", page).params.id;
    $$renderer2.push(`<div class="max-w-2xl mx-auto"><div class="flex items-center justify-between mb-6"><h1 class="text-2xl font-bold text-gray-900">Edit Contact</h1> <a${attr("href", `/contacts/${stringify(contactId)}`)} class="text-gray-600 hover:text-gray-900 text-sm">← Back to Contact</a></div> `);
    {
      $$renderer2.push("<!--[-->");
      FormSkeleton($$renderer2, { fields: 6 });
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
