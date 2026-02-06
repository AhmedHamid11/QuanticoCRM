import { W as store_get, _ as unsubscribe_stores } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import "../../../../../chunks/auth.svelte.js";
import { D as DetailSkeleton } from "../../../../../chunks/DetailSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).params.id;
    $$renderer2.push(`<div class="space-y-6">`);
    {
      $$renderer2.push("<!--[-->");
      DetailSkeleton($$renderer2, {});
    }
    $$renderer2.push(`<!--]--></div> `);
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
