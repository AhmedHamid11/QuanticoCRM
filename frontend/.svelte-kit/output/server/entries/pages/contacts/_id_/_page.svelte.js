import { W as store_get, _ as unsubscribe_stores } from "../../../../chunks/index.js";
import { p as page } from "../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { e as escape_html } from "../../../../chunks/attributes.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import { D as DetailSkeleton } from "../../../../chunks/DetailSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).params.id;
    let relatedListConfigs = [];
    relatedListConfigs.filter((c) => c.enabled).sort((a, b) => a.sortOrder - b.sortOrder);
    $$renderer2.push(`<div class="space-y-6"><nav class="text-sm text-gray-500"><a href="/contacts" class="hover:text-gray-700">Contacts</a> <span class="mx-2">/</span> <span class="text-gray-900">${escape_html("Loading...")}</span></nav> `);
    {
      $$renderer2.push("<!--[-->");
      DetailSkeleton($$renderer2, { sections: 2, fieldsPerSection: 4 });
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
