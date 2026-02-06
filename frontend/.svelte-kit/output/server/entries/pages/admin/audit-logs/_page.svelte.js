import { X as ensure_array_like } from "../../../../chunks/index.js";
import "../../../../chunks/auth.svelte.js";
import { a as attr, e as escape_html } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let eventTypes = [];
    let filters = { eventType: "", dateFrom: "", dateTo: "" };
    let verifyingChain = false;
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div><h1 class="text-2xl font-bold text-gray-900">Audit Logs</h1> <p class="mt-1 text-sm text-gray-500">Security events and compliance audit trail</p></div> <div class="flex items-center space-x-3"><button class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path></svg> Export CSV</button> <button class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path></svg> Export JSON</button> <button${attr("disabled", verifyingChain, true)} class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path></svg> ${escape_html("Verify Chain")}</button> <a href="/admin" class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"></path></svg> Back to Admin</a></div></div> <div class="bg-white shadow rounded-lg p-6"><h2 class="text-lg font-medium text-gray-900 mb-4">Filters</h2> <div class="grid grid-cols-1 md:grid-cols-4 gap-4"><div><label for="eventType" class="block text-sm font-medium text-gray-700 mb-1">Event Type</label> `);
    $$renderer2.select(
      {
        id: "eventType",
        value: filters.eventType,
        class: "block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Events`);
        });
        $$renderer3.push(`<!--[-->`);
        const each_array = ensure_array_like(eventTypes);
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let type = each_array[$$index];
          $$renderer3.option({ value: type.value }, ($$renderer4) => {
            $$renderer4.push(`${escape_html(type.label)}`);
          });
        }
        $$renderer3.push(`<!--]-->`);
      }
    );
    $$renderer2.push(`</div> <div><label for="dateFrom" class="block text-sm font-medium text-gray-700 mb-1">From Date</label> <input type="date" id="dateFrom"${attr("value", filters.dateFrom)} class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"/></div> <div><label for="dateTo" class="block text-sm font-medium text-gray-700 mb-1">To Date</label> <input type="date" id="dateTo"${attr("value", filters.dateTo)} class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 text-sm"/></div> <div class="flex items-end space-x-2"><button class="flex-1 inline-flex justify-center items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-600/90">Apply</button> <button class="flex-1 inline-flex justify-center items-center px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50">Reset</button></div></div></div> <div class="bg-white shadow rounded-lg overflow-hidden">`);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="flex items-center justify-center py-12"><div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div></div>`);
    }
    $$renderer2.push(`<!--]--></div></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
