import { Y as attr_class, Z as stringify, W as store_get, X as ensure_array_like, _ as unsubscribe_stores } from "../../../../chunks/index.js";
import { p as page } from "../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { e as escape_html, a as attr } from "../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import "../../../../chunks/auth.svelte.js";
import { a as getEntityNameFromPath } from "../../../../chunks/navigation.svelte.js";
/* empty css                                                       */
import "clsx";
function FlowButton($$renderer, $$props) {
  let {
    label = "Run Flow",
    variant = "primary",
    size = "md"
  } = $$props;
  const sizeClasses = {
    sm: "px-2.5 py-1.5 text-xs",
    md: "px-3 py-2 text-sm",
    lg: "px-4 py-2 text-base"
  };
  const variantClasses = {
    primary: "bg-blue-600 text-white hover:bg-blue-600/90 focus:ring-blue-500 border-transparent",
    secondary: "bg-white text-gray-700 hover:bg-gray-50 focus:ring-blue-500 border-gray-300",
    ghost: "bg-transparent text-gray-600 hover:bg-gray-100 focus:ring-gray-500 border-transparent"
  };
  $$renderer.push(`<button type="button"${attr_class(`inline-flex items-center font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-offset-2 ${stringify(sizeClasses[size])} ${stringify(variantClasses[variant])}`)}><svg class="w-4 h-4 mr-1.5 -ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg> ${escape_html(label)}</button> `);
  {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]-->`);
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("tab");
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
    let relatedListConfigs = [];
    let entityFlows = [];
    relatedListConfigs.filter((c) => c.enabled).sort((a, b) => a.sortOrder - b.sortOrder);
    $$renderer2.push(`<div class="space-y-6"><div class="flex justify-between items-start"><div><nav class="text-sm text-gray-500 mb-2"><a${attr("href", `/${stringify(
      // Reload all data when entity or record changes (handles navigation between entities)
      // Track these reactive values to trigger reload on navigation
      // Reset state
      // Load all data
      // If no layout configured, fallback to showing all fields
      entitySlug
    )}`)} class="hover:text-gray-700">${escape_html(entityName + "s")}</a> <span class="mx-2">/</span> <span class="text-gray-900">${escape_html(recordId)}</span></nav> <div class="flex items-center gap-3">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <h1 class="text-2xl font-bold text-gray-900">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`Loading...`);
    }
    $$renderer2.push(`<!--]--></h1></div></div> <div class="flex gap-2"><a${attr("href", `/admin/entity-manager/${stringify(entityName)}`)} class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors" title="Entity Settings"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path></svg></a> <!--[-->`);
    const each_array = ensure_array_like(entityFlows);
    for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
      let flow = each_array[$$index];
      FlowButton($$renderer2, {
        flowId: flow.id,
        label: flow.buttonLabel || flow.name,
        variant: "secondary",
        refreshOnComplete: flow.refreshOnComplete
      });
    }
    $$renderer2.push(`<!--]--> <a${attr("href", `/${stringify(entitySlug)}/${stringify(recordId)}/edit`)} class="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors">Edit</a> <button class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors">Delete</button></div></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[-->");
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--> `);
      {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
