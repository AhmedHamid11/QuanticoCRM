import { W as store_get, _ as unsubscribe_stores, X as ensure_array_like, Y as attr_class, Z as stringify } from "../../../../../../chunks/index.js";
import { p as page } from "../../../../../../chunks/stores.js";
import { g as get } from "../../../../../../chunks/api.js";
import { t as toast } from "../../../../../../chunks/toast.svelte.js";
import { e as escape_html, a as attr } from "../../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let logs = [];
    let loading = true;
    let error = null;
    let currentPage = 1;
    let pageSize = 20;
    let total = 0;
    let totalPages = 0;
    let statusFilter = "";
    let eventFilter = "";
    const tripwireId = store_get($$store_subs ??= {}, "$page", page).params.id;
    async function loadLogs() {
      try {
        loading = true;
        error = null;
        const params = new URLSearchParams({
          page: currentPage.toString(),
          pageSize: pageSize.toString(),
          sortBy: "executed_at",
          sortDir: "desc"
        });
        if (statusFilter) ;
        if (eventFilter) ;
        const result = await get(`/tripwires/${tripwireId}/logs?${params}`);
        logs = result.data;
        total = result.total;
        totalPages = result.totalPages;
      } catch (e) {
        error = e instanceof Error ? e.message : "Failed to load logs";
        toast.error(error);
      } finally {
        loading = false;
      }
    }
    function handleFilterChange() {
      currentPage = 1;
      loadLogs();
    }
    function formatDateTime(dateStr) {
      return new Date(dateStr).toLocaleString();
    }
    function getStatusColor(status) {
      switch (status) {
        case "success":
          return "bg-green-100 text-green-800";
        case "failed":
          return "bg-red-100 text-red-800";
        case "timeout":
          return "bg-yellow-100 text-yellow-800";
        default:
          return "bg-gray-100 text-gray-800";
      }
    }
    function getEventColor(event) {
      switch (event) {
        case "CREATE":
          return "bg-blue-100 text-blue-800";
        case "UPDATE":
          return "bg-purple-100 text-purple-800";
        case "DELETE":
          return "bg-red-100 text-red-800";
        default:
          return "bg-gray-100 text-gray-800";
      }
    }
    $$renderer2.push(`<div class="space-y-4"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Execution Logs</h1> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> <a href="/admin/tripwires" class="text-sm text-gray-600 hover:text-gray-900">← Back to Tripwires</a></div> <div class="flex gap-4 flex-wrap">`);
    $$renderer2.select(
      {
        value: statusFilter,
        onchange: handleFilterChange,
        class: "px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Statuses`);
        });
        $$renderer3.option({ value: "success" }, ($$renderer4) => {
          $$renderer4.push(`Success`);
        });
        $$renderer3.option({ value: "failed" }, ($$renderer4) => {
          $$renderer4.push(`Failed`);
        });
        $$renderer3.option({ value: "timeout" }, ($$renderer4) => {
          $$renderer4.push(`Timeout`);
        });
      }
    );
    $$renderer2.push(` `);
    $$renderer2.select(
      {
        value: eventFilter,
        onchange: handleFilterChange,
        class: "px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Events`);
        });
        $$renderer3.option({ value: "CREATE" }, ($$renderer4) => {
          $$renderer4.push(`CREATE`);
        });
        $$renderer3.option({ value: "UPDATE" }, ($$renderer4) => {
          $$renderer4.push(`UPDATE`);
        });
        $$renderer3.option({ value: "DELETE" }, ($$renderer4) => {
          $$renderer4.push(`DELETE`);
        });
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
        if (logs.length === 0) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="text-center py-12 text-gray-500">No execution logs found.</div>`);
        } else {
          $$renderer2.push("<!--[!-->");
          $$renderer2.push(`<div class="bg-white shadow rounded-lg overflow-hidden"><table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Executed At</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Event</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Entity / Record</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Response</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Duration</th></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
          const each_array = ensure_array_like(logs);
          for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
            let log = each_array[$$index];
            $$renderer2.push(`<tr class="hover:bg-gray-50"><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(formatDateTime(log.executedAt))}</td><td class="px-6 py-4 whitespace-nowrap"><span${attr_class(`px-2 py-1 text-xs font-medium rounded-full ${stringify(getEventColor(log.eventType))}`)}>${escape_html(log.eventType)}</span></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(log.entityType)} / ${escape_html(log.recordId)}</td><td class="px-6 py-4 whitespace-nowrap"><span${attr_class(`px-2 py-1 text-xs font-medium rounded-full ${stringify(getStatusColor(log.status))}`)}>${escape_html(log.status)}</span></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">`);
            if (log.responseCode) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`HTTP ${escape_html(log.responseCode)}`);
            } else {
              $$renderer2.push("<!--[!-->");
              if (log.errorMessage) {
                $$renderer2.push("<!--[-->");
                $$renderer2.push(`<span class="text-red-600"${attr("title", log.errorMessage)}>${escape_html(log.errorMessage.length > 30 ? log.errorMessage.substring(0, 30) + "..." : log.errorMessage)}</span>`);
              } else {
                $$renderer2.push("<!--[!-->");
                $$renderer2.push(`-`);
              }
              $$renderer2.push(`<!--]-->`);
            }
            $$renderer2.push(`<!--]--></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">`);
            if (log.durationMs !== void 0) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`${escape_html(log.durationMs)}ms`);
            } else {
              $$renderer2.push("<!--[!-->");
              $$renderer2.push(`-`);
            }
            $$renderer2.push(`<!--]--></td></tr>`);
          }
          $$renderer2.push(`<!--]--></tbody></table></div> `);
          if (totalPages > 1) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<div class="flex justify-between items-center"><p class="text-sm text-gray-700">Showing ${escape_html((currentPage - 1) * pageSize + 1)} to ${escape_html(Math.min(currentPage * pageSize, total))} of ${escape_html(total)} results</p> <div class="flex gap-2"><button${attr("disabled", currentPage === 1, true)} class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50">Previous</button> <span class="px-3 py-1 text-sm text-gray-700">Page ${escape_html(currentPage)} of ${escape_html(totalPages)}</span> <button${attr("disabled", currentPage === totalPages, true)} class="px-3 py-1 border border-gray-300 rounded-md text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50">Next</button></div></div>`);
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
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
