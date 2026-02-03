import { X as ensure_array_like, Y as attr_class, Z as stringify } from "../../../chunks/index.js";
import { T as TableSkeleton } from "../../../chunks/TableSkeleton.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import { a as attr, e as escape_html } from "../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../chunks/state.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let search = "";
    let pageSize = 20;
    let layoutFields = [];
    let listViews = [];
    let selectedListView = null;
    $$renderer2.push(`<div class="space-y-4"><div class="flex justify-between items-center"><h1 class="text-2xl font-bold text-gray-900">Accounts</h1> <a href="/accounts/new" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700">+ New Account</a></div> <div class="flex flex-wrap gap-3 items-start"><div class="flex-1 min-w-[200px] relative"><input type="text"${attr("value", search)} placeholder="Search accounts..." class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/> <svg class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clip-rule="evenodd"></path></svg> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> <div class="relative"><select class="pl-3 pr-8 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500 bg-white text-sm">`);
    $$renderer2.option({ value: "", selected: !selectedListView }, ($$renderer3) => {
      $$renderer3.push(`All Records`);
    });
    $$renderer2.push(`<!--[-->`);
    const each_array = ensure_array_like(listViews);
    for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
      let view = each_array[$$index];
      $$renderer2.option({ value: view.id, selected: selectedListView?.id === view.id }, ($$renderer3) => {
        $$renderer3.push(`${escape_html(view.name)} ${escape_html(view.isDefault ? "(Default)" : "")}`);
      });
    }
    $$renderer2.push(`<!--]--></select></div> <button${attr_class(`px-3 py-2 border rounded-md text-sm flex items-center gap-2 ${stringify("border-gray-300 text-gray-700 hover:bg-gray-50")}`)}><svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M2.628 1.601C5.028 1.206 7.49 1 10 1s4.973.206 7.372.601a.75.75 0 01.628.74v2.288a2.25 2.25 0 01-.659 1.59l-4.682 4.683a2.25 2.25 0 00-.659 1.59v3.037c0 .684-.31 1.33-.844 1.757l-1.937 1.55A.75.75 0 018 18.25v-5.757a2.25 2.25 0 00-.659-1.591L2.659 6.22A2.25 2.25 0 012 4.629V2.34a.75.75 0 01.628-.74z" clip-rule="evenodd"></path></svg> Filter `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></button> <button class="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50 flex items-center gap-2"><svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z"></path></svg> Save View</button> `);
    if (listViews.length > 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<button class="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50">Manage Views</button>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[-->");
      TableSkeleton($$renderer2, { rows: pageSize, columns: layoutFields.length || 4 });
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
