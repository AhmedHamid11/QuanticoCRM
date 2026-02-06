import { X as ensure_array_like, Z as stringify, Y as attr_class } from "../../../../chunks/index.js";
import { g as get } from "../../../../chunks/api.js";
import { t as toast } from "../../../../chunks/toast.svelte.js";
import { a as attr, e as escape_html } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let tripwires = [];
    let entities = [];
    let loading = true;
    let error = null;
    let search = "";
    let entityFilter = "";
    let page = 1;
    let pageSize = 20;
    let total = 0;
    let totalPages = 0;
    let sortBy = "created_at";
    let sortDir = "desc";
    async function loadTripwires() {
      try {
        loading = true;
        error = null;
        const params = new URLSearchParams({
          page: page.toString(),
          pageSize: pageSize.toString(),
          sortBy,
          sortDir
        });
        if (search) ;
        if (entityFilter) ;
        const result = await get(`/tripwires?${params}`);
        tripwires = result.data;
        total = result.total;
        totalPages = result.totalPages;
      } catch (e) {
        error = e instanceof Error ? e.message : "Failed to load tripwires";
        toast.error(error);
      } finally {
        loading = false;
      }
    }
    function handleFilterChange() {
      page = 1;
      loadTripwires();
    }
    function truncateUrl(url, maxLen = 40) {
      if (url.length <= maxLen) return url;
      return url.substring(0, maxLen) + "...";
    }
    function getConditionsSummary(tw) {
      if (!tw.conditions || tw.conditions.length === 0) return "No conditions";
      const count = tw.conditions.length;
      return `${count} condition${count > 1 ? "s" : ""} (${tw.conditionLogic})`;
    }
    $$renderer2.push(`<div class="space-y-4"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Tripwires</h1> <p class="text-sm text-gray-500 mt-1">Webhook triggers that fire when entity records are created, updated, or deleted</p></div> <div class="flex gap-2"><a href="/admin/settings/webhooks" class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50">Webhook Settings</a> <a href="/admin/tripwires/new" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90">+ New Tripwire</a></div></div> <div class="flex gap-4 flex-wrap"><div class="flex-1 min-w-64 relative"><input type="text"${attr("value", search)} placeholder="Search tripwires..." class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/> <svg class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clip-rule="evenodd"></path></svg></div> `);
    $$renderer2.select(
      {
        value: entityFilter,
        onchange: handleFilterChange,
        class: "px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Entities`);
        });
        $$renderer3.push(`<!--[-->`);
        const each_array = ensure_array_like(entities);
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let entity = each_array[$$index];
          $$renderer3.option({ value: entity.name }, ($$renderer4) => {
            $$renderer4.push(`${escape_html(entity.label)}`);
          });
        }
        $$renderer3.push(`<!--]-->`);
      }
    );
    $$renderer2.push(`</div> `);
    if (loading) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (error) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="text-center py-12 text-red-500">${escape_html(error)}</div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        if (tripwires.length === 0) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="text-center py-12 text-gray-500">No tripwires found. <a href="/admin/tripwires/new" class="text-blue-600 hover:underline">Create one</a></div>`);
        } else {
          $$renderer2.push("<!--[!-->");
          $$renderer2.push(`<div class="bg-white shadow rounded-lg overflow-hidden"><table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Name `);
          {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Entity `);
          {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Endpoint</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Conditions</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Status `);
          {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
          const each_array_1 = ensure_array_like(tripwires);
          for (let $$index_1 = 0, $$length = each_array_1.length; $$index_1 < $$length; $$index_1++) {
            let tw = each_array_1[$$index_1];
            $$renderer2.push(`<tr class="hover:bg-gray-50"><td class="px-6 py-4 whitespace-nowrap"><a${attr("href", `/admin/tripwires/${stringify(tw.id)}`)} class="text-blue-600 hover:underline font-medium">${escape_html(tw.name)}</a> `);
            if (tw.description) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<p class="text-xs text-gray-500 mt-0.5">${escape_html(tw.description)}</p>`);
            } else {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]--></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(tw.entityType)}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500"${attr("title", tw.endpointUrl)}>${escape_html(truncateUrl(tw.endpointUrl))}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(getConditionsSummary(tw))}</td><td class="px-6 py-4 whitespace-nowrap"><button${attr_class(`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${stringify(tw.enabled ? "bg-blue-600" : "bg-gray-200")}`)}><span${attr_class(`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${stringify(tw.enabled ? "translate-x-5" : "translate-x-0")}`)}></span></button></td><td class="px-6 py-4 whitespace-nowrap text-right text-sm"><a${attr("href", `/admin/tripwires/${stringify(tw.id)}/logs`)} class="text-gray-600 hover:underline mr-4">Logs</a> <a${attr("href", `/admin/tripwires/${stringify(tw.id)}`)} class="text-blue-600 hover:underline mr-4">Edit</a> <button class="text-red-600 hover:underline">Delete</button></td></tr>`);
          }
          $$renderer2.push(`<!--]--></tbody></table></div> `);
          if (totalPages > 1) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<div class="flex justify-between items-center"><p class="text-sm text-gray-700">Showing ${escape_html((page - 1) * pageSize + 1)} to ${escape_html(Math.min(page * pageSize, total))} of ${escape_html(total)} results</p> <div class="flex gap-2"><button${attr("disabled", page === 1, true)} class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50">Previous</button> <span class="px-3 py-1 text-sm text-gray-700">Page ${escape_html(page)} of ${escape_html(totalPages)}</span> <button${attr("disabled", page === totalPages, true)} class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50">Next</button></div></div>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]-->`);
        }
        $$renderer2.push(`<!--]-->`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
