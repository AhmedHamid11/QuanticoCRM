import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../../chunks/index.js";
import { p as page } from "../../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { a as attr, e as escape_html } from "../../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import "../../../../../chunks/auth.svelte.js";
import { a as getEntityNameFromPath } from "../../../../../chunks/navigation.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let entitySlug = store_get($$store_subs ??= {}, "$page", page).params.entity;
    let entityName = getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug);
    let recordId = store_get($$store_subs ??= {}, "$page", page).params.id;
    function toPascalCase(slug) {
      let singular = slug;
      if (slug.endsWith("s") && slug.length > 1) {
        singular = slug.slice(0, -1);
      }
      return singular.charAt(0).toUpperCase() + singular.slice(1);
    }
    let formData = {};
    let $$settled = true;
    let $$inner_renderer;
    function $$render_inner($$renderer3) {
      $$renderer3.push(`<div class="space-y-6"><div><nav class="text-sm text-gray-500 mb-2"><a${attr("href", `/${stringify(entitySlug)}`)} class="hover:text-gray-700">${escape_html(entityName + "s")}</a> <span class="mx-2">/</span> <a${attr("href", `/${stringify(entitySlug)}/${stringify(recordId)}`)} class="hover:text-gray-700">${escape_html(formData.name || recordId)}</a> <span class="mx-2">/</span> <span class="text-gray-900">Edit</span></nav> <h1 class="text-2xl font-bold text-gray-900">Edit ${escape_html(entityName)}</h1></div> `);
      {
        $$renderer3.push("<!--[-->");
        $$renderer3.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
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
