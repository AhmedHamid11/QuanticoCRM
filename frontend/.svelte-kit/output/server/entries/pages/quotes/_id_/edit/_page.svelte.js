import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { a as attr } from "../../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import "../../../../../chunks/auth.svelte.js";
import { F as FormSkeleton } from "../../../../../chunks/FormSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let quoteId = store_get($$store_subs ??= {}, "$page", page).params.id;
    let $$settled = true;
    let $$inner_renderer;
    function $$render_inner($$renderer3) {
      $$renderer3.push(`<div class="max-w-4xl mx-auto"><div class="flex items-center justify-between mb-6"><h1 class="text-2xl font-bold text-gray-900">Edit Quote</h1> <a${attr("href", `/quotes/${stringify(quoteId)}`)} class="text-gray-600 hover:text-gray-900 text-sm">← Back to Quote</a></div> `);
      {
        $$renderer3.push("<!--[-->");
        FormSkeleton($$renderer3, { fields: 6 });
      }
      $$renderer3.push(`<!--]--></div>`);
    }
    do {
      $$settled = true;
      $$inner_renderer = $$renderer2.copy();
      $$render_inner($$inner_renderer);
    } while (!$$settled);
    $$renderer2.subsume($$inner_renderer);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
