import { W as store_get, _ as unsubscribe_stores } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let fields = [];
    let layoutFields = [];
    store_get($$store_subs ??= {}, "$page", page).params.id;
    layoutFields.map((fieldName) => fields.find((f) => f.name === fieldName)).filter((f) => f !== void 0 && f.name !== "id" && !f.isReadOnly);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
    }
    $$renderer2.push(`<!--]-->`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
