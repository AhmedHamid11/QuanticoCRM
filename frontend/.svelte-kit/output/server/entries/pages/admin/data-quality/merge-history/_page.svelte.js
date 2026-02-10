import { X as ensure_array_like, Y as attr_class, Z as stringify } from "../../../../../chunks/index.js";
import { m as mergeHistory } from "../../../../../chunks/data-quality.js";
import { t as toast } from "../../../../../chunks/toast.svelte.js";
import { e as escape_html, a as attr } from "../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let entries = [];
    let loading = true;
    let page = 1;
    let total = 0;
    let pageSize = 20;
    let entityFilter = "";
    let undoingSnapshot = null;
    const entityTypes = [
      "",
      "Contact",
      "Account",
      "Lead",
      "Opportunity",
      "Task",
      "Meeting",
      "Call",
      "Email"
    ];
    async function loadHistory() {
      loading = true;
      try {
        const params = { page, pageSize };
        if (entityFilter) ;
        const response = await mergeHistory(params);
        entries = response.data || [];
        total = response.total || 0;
      } catch (err) {
        toast.error(`Failed to load merge history: ${err.message}`);
      } finally {
        loading = false;
      }
    }
    function canUndo(entry) {
      return entry.canUndo;
    }
    function getUndoTooltip(entry) {
      if (!entry.canUndo) {
        const expiresAt = new Date(entry.expiresAt);
        if (expiresAt <= /* @__PURE__ */ new Date()) return "Expired (30-day window passed)";
        return "Already undone";
      }
      return "Undo this merge";
    }
    function getStatusText(entry) {
      if (!entry.canUndo) {
        const expiresAt = new Date(entry.expiresAt);
        if (expiresAt <= /* @__PURE__ */ new Date()) return "Permanent";
        return "Undone";
      }
      return "Active";
    }
    function getStatusClass(entry) {
      if (!entry.canUndo) {
        const expiresAt = new Date(entry.expiresAt);
        if (expiresAt <= /* @__PURE__ */ new Date()) return "bg-gray-100 text-gray-600";
        return "bg-gray-100 text-gray-800";
      }
      return "bg-green-100 text-green-800";
    }
    function formatDate(dateStr) {
      try {
        const date = new Date(dateStr);
        return date.toLocaleDateString("en-US", {
          year: "numeric",
          month: "short",
          day: "numeric",
          hour: "2-digit",
          minute: "2-digit"
        });
      } catch {
        return dateStr;
      }
    }
    function handleEntityFilterChange() {
      page = 1;
      loadHistory();
    }
    const totalPages = Math.ceil(total / pageSize);
    $$renderer2.push(`<div class="space-y-6"><div><h1 class="text-2xl font-bold text-gray-900">Merge History</h1> <p class="mt-1 text-sm text-gray-500">View recent merge operations. You can undo merges within 30 days.</p></div> <div class="bg-white rounded-lg shadow p-4"><div class="flex items-center gap-4"><label for="history-entity-filter" class="text-sm font-medium text-gray-700">Entity Type:</label> `);
    $$renderer2.select(
      {
        id: "history-entity-filter",
        name: "history-entity-filter",
        value: entityFilter,
        onchange: handleEntityFilterChange,
        class: "border border-gray-300 rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All`);
        });
        $$renderer3.push(`<!--[-->`);
        const each_array = ensure_array_like(entityTypes.slice(1));
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let type = each_array[$$index];
          $$renderer3.option({ value: type }, ($$renderer4) => {
            $$renderer4.push(`${escape_html(type)}`);
          });
        }
        $$renderer3.push(`<!--]-->`);
      }
    );
    $$renderer2.push(`</div></div> <div class="bg-white rounded-lg shadow overflow-hidden">`);
    if (loading) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="p-12 text-center"><div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div> <p class="mt-4 text-gray-600">Loading merge history...</p></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (entries.length === 0) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="p-12 text-center"><p class="text-gray-500 text-lg">No merge operations recorded yet.</p> `);
        {
          $$renderer2.push("<!--[!-->");
        }
        $$renderer2.push(`<!--]--></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Date</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Entity Type</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Survivor ID</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Duplicates Merged</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Merged By</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
        const each_array_1 = ensure_array_like(entries);
        for (let $$index_1 = 0, $$length = each_array_1.length; $$index_1 < $$length; $$index_1++) {
          let entry = each_array_1[$$index_1];
          $$renderer2.push(`<tr class="hover:bg-gray-50"><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${escape_html(formatDate(entry.createdAt))}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${escape_html(entry.entityType)}</td><td class="px-6 py-4 whitespace-nowrap text-sm"><a${attr("href", `/${stringify(entry.entityType.toLowerCase())}s/${stringify(entry.survivorId)}`)} class="text-blue-600 hover:text-blue-800">${escape_html(entry.survivorName || entry.survivorId.substring(0, 8) + "...")}</a></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${escape_html(entry.duplicateIds.length)}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-mono">${escape_html(entry.mergedById.substring(0, 8))}...</td><td class="px-6 py-4 whitespace-nowrap"><span${attr_class(`px-2 py-1 text-xs font-medium rounded ${stringify(getStatusClass(entry))}`)}>${escape_html(getStatusText(entry))}</span></td><td class="px-6 py-4 whitespace-nowrap text-sm">`);
          if (canUndo(entry)) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button${attr("disabled", undoingSnapshot === entry.snapshotId, true)}${attr("title", getUndoTooltip(entry))} class="px-3 py-1 bg-yellow-600 text-white rounded hover:bg-yellow-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition">${escape_html(undoingSnapshot === entry.snapshotId ? "Undoing..." : "Undo")}</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
            $$renderer2.push(`<span${attr("title", getUndoTooltip(entry))} class="text-gray-400 cursor-not-allowed">Undo</span>`);
          }
          $$renderer2.push(`<!--]--></td></tr>`);
        }
        $$renderer2.push(`<!--]--></tbody></table></div> `);
        if (totalPages > 1) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="bg-gray-50 px-6 py-4 border-t border-gray-200"><div class="flex items-center justify-between"><div class="text-sm text-gray-700">Showing <strong>${escape_html((page - 1) * pageSize + 1)}</strong> to <strong>${escape_html(Math.min(page * pageSize, total))}</strong> of <strong>${escape_html(total)}</strong> results</div> <div class="flex gap-2"><button${attr("disabled", page === 1, true)} class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed">Previous</button> `);
          if (page > 2) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100">1</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> `);
          if (page > 3) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="px-3 py-1 text-gray-500">...</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> `);
          if (page > 1) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100">${escape_html(page - 1)}</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> <button class="px-3 py-1 border border-blue-500 bg-blue-50 rounded text-sm font-medium text-blue-600">${escape_html(page)}</button> `);
          if (page < totalPages) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100">${escape_html(page + 1)}</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> `);
          if (page < totalPages - 2) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="px-3 py-1 text-gray-500">...</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> `);
          if (page < totalPages - 1) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100">${escape_html(totalPages)}</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> <button${attr("disabled", page === totalPages, true)} class="px-3 py-1 border border-gray-300 rounded text-sm font-medium text-gray-700 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed">Next</button></div></div></div>`);
        } else {
          $$renderer2.push("<!--[!-->");
        }
        $$renderer2.push(`<!--]-->`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div></div>`);
  });
}
export {
  _page as default
};
