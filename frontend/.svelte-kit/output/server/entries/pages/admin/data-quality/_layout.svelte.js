import { X as ensure_array_like, Y as attr_class, Z as stringify, W as store_get, _ as unsubscribe_stores } from "../../../../chunks/index.js";
import { p as page } from "../../../../chunks/stores.js";
import { a as attr, e as escape_html } from "../../../../chunks/attributes.js";
function _layout($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let { children } = $$props;
    const tabs = [
      {
        id: "rules",
        label: "Duplicate Rules",
        href: "/admin/data-quality/duplicate-rules"
      },
      {
        id: "queue",
        label: "Review Queue",
        href: "/admin/data-quality/review-queue"
      },
      {
        id: "history",
        label: "Merge History",
        href: "/admin/data-quality/merge-history"
      },
      {
        id: "scans",
        label: "Scan Jobs",
        href: "/admin/data-quality/scan-jobs"
      }
    ];
    function isActive(href) {
      return store_get($$store_subs ??= {}, "$page", page).url.pathname === href || store_get($$store_subs ??= {}, "$page", page).url.pathname.startsWith(href + "/");
    }
    $$renderer2.push(`<div class="space-y-6"><div><h1 class="text-2xl font-bold text-gray-900">Data Quality</h1> <p class="mt-1 text-sm text-gray-500">Manage duplicate detection rules, review queue, and merge operations</p></div> <div class="bg-white shadow rounded-lg"><div class="border-b border-gray-200"><nav class="-mb-px flex space-x-8 px-6" aria-label="Tabs"><!--[-->`);
    const each_array = ensure_array_like(tabs);
    for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
      let tab = each_array[$$index];
      $$renderer2.push(`<a${attr("href", tab.href)}${attr_class(`border-b-2 py-4 px-1 text-sm font-medium transition-colors ${stringify(isActive(tab.href) ? "border-blue-500 text-blue-600" : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300")}`)}>${escape_html(tab.label)}</a>`);
    }
    $$renderer2.push(`<!--]--></nav></div> <div class="p-6">`);
    children($$renderer2);
    $$renderer2.push(`<!----></div></div></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _layout as default
};
