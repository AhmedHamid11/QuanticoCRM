import { X as ensure_array_like, Y as attr_class, Z as stringify } from "../../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import { e as escape_html, a as attr } from "../../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import { l as listPendingAlerts } from "../../../../../chunks/data-quality.js";
import "../../../../../chunks/auth.svelte.js";
import { t as toast } from "../../../../../chunks/toast.svelte.js";
function formatConfidence(score) {
  return `${Math.round(score * 100)}%`;
}
function getConfidenceBadgeClass(tier) {
  switch (tier) {
    case "high":
      return "bg-red-100 text-red-800 border-red-200";
    case "medium":
      return "bg-yellow-100 text-yellow-800 border-yellow-200";
    case "low":
      return "bg-blue-100 text-blue-800 border-blue-200";
    default:
      return "bg-gray-100 text-gray-800 border-gray-200";
  }
}
function getBannerClass(confidence) {
  switch (confidence) {
    case "high":
      return "bg-red-50 border-red-200 text-red-800";
    case "medium":
      return "bg-yellow-50 border-yellow-200 text-yellow-800";
    case "low":
      return "bg-blue-50 border-blue-200 text-blue-800";
    default:
      return "bg-gray-50 border-gray-200 text-gray-800";
  }
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let alerts = [];
    let loading = true;
    let entityFilter = "";
    let currentPage = 1;
    let total = 0;
    let pageSize = 20;
    let selectedIds = /* @__PURE__ */ new Set();
    let showBulkBar = selectedIds.size > 0;
    let bulkProcessing = false;
    async function loadAlerts() {
      loading = true;
      try {
        const response = await listPendingAlerts({
          entityType: entityFilter || void 0,
          page: currentPage,
          pageSize
        });
        alerts = response.data || [];
        total = response.total || 0;
      } catch (error) {
        toast.error(`Failed to load alerts: ${error.message || "Unknown error"}`);
      } finally {
        loading = false;
      }
    }
    function handleFilterChange(event) {
      const target = event.target;
      entityFilter = target.value;
      currentPage = 1;
      selectedIds = /* @__PURE__ */ new Set();
      loadAlerts();
    }
    const totalPages = Math.ceil(total / pageSize);
    const hasNextPage = currentPage < totalPages;
    const hasPrevPage = currentPage > 1;
    $$renderer2.push(`<div class="p-6"><div class="mb-6 flex items-center justify-between"><div class="flex items-center gap-4"><h1 class="text-2xl font-semibold text-gray-900">Review Queue</h1> <span class="rounded-full bg-blue-100 px-3 py-1 text-sm font-medium text-blue-800">${escape_html(total)} pending</span></div> <div class="flex items-center gap-4"><label for="entity-filter" class="sr-only">Filter by entity type</label> `);
    $$renderer2.select(
      {
        id: "entity-filter",
        name: "entity-filter",
        onchange: handleFilterChange,
        value: entityFilter,
        class: "rounded-lg border border-gray-300 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Entity Types`);
        });
        $$renderer3.option({ value: "Contact" }, ($$renderer4) => {
          $$renderer4.push(`Contact`);
        });
        $$renderer3.option({ value: "Account" }, ($$renderer4) => {
          $$renderer4.push(`Account`);
        });
        $$renderer3.option({ value: "Lead" }, ($$renderer4) => {
          $$renderer4.push(`Lead`);
        });
        $$renderer3.option({ value: "Opportunity" }, ($$renderer4) => {
          $$renderer4.push(`Opportunity`);
        });
      }
    );
    $$renderer2.push(` `);
    if (totalPages > 1) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="flex items-center gap-2"><button${attr("disabled", !hasPrevPage, true)} class="rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50">Previous</button> <span class="text-sm text-gray-600">Page ${escape_html(currentPage)} of ${escape_html(totalPages)}</span> <button${attr("disabled", !hasNextPage, true)} class="rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50">Next</button></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div></div> `);
    if (loading) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="space-y-4"><!--[-->`);
      const each_array = ensure_array_like([1, 2, 3]);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        each_array[$$index];
        $$renderer2.push(`<div class="animate-pulse rounded-lg border border-gray-200 bg-white p-6"><div class="h-4 w-1/3 rounded bg-gray-200"></div> <div class="mt-4 h-20 rounded bg-gray-200"></div> <div class="mt-4 flex gap-2"><div class="h-8 w-20 rounded bg-gray-200"></div> <div class="h-8 w-24 rounded bg-gray-200"></div> <div class="h-8 w-16 rounded bg-gray-200"></div></div></div>`);
      }
      $$renderer2.push(`<!--]--></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (alerts.length === 0) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="flex flex-col items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-white py-16"><svg class="mb-4 h-16 w-16 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg> <h3 class="mb-2 text-lg font-medium text-gray-900">No duplicates found</h3> <p class="mb-6 text-sm text-gray-500">All clear! No duplicate records detected in your data.</p> <button class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">Run a scan</button></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="space-y-4">`);
        if (alerts.length > 0) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="flex items-center gap-2 rounded-lg bg-gray-50 px-4 py-2"><input id="select-all" name="select-all" type="checkbox"${attr("checked", selectedIds.size === alerts.length && alerts.length > 0, true)} class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"/> <label for="select-all" class="text-sm text-gray-700">${escape_html(selectedIds.size > 0 ? `${selectedIds.size} selected` : "Select all")}</label></div>`);
        } else {
          $$renderer2.push("<!--[!-->");
        }
        $$renderer2.push(`<!--]--> <!--[-->`);
        const each_array_1 = ensure_array_like(alerts);
        for (let $$index_3 = 0, $$length = each_array_1.length; $$index_3 < $$length; $$index_3++) {
          let alert = each_array_1[$$index_3];
          const borderClass = getBannerClass(alert.highestConfidence);
          const badgeClass = getConfidenceBadgeClass(alert.highestConfidence);
          const confidenceScore = alert.matches[0]?.matchResult?.score || 0;
          $$renderer2.push(`<div${attr_class(`rounded-lg border-l-4 ${stringify(borderClass)} border-r border-t border-b border-gray-200 bg-white p-6 shadow-sm`)}><div class="flex items-start gap-4"><input type="checkbox"${attr("id", `alert-${stringify(alert.id)}`)}${attr("name", `alert-${stringify(alert.id)}`)}${attr("checked", selectedIds.has(alert.id), true)}${attr("aria-label", `Select ${stringify(alert.entityType)} ${stringify(alert.recordId)}`)} class="mt-1 h-5 w-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"/> <div class="flex-1"><div class="mb-4 flex items-start justify-between"><div><div class="flex items-center gap-2"><h3 class="text-lg font-medium text-gray-900">${escape_html(alert.recordName || alert.recordId)}</h3> <span class="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-600">${escape_html(alert.entityType)}</span> <span${attr_class(`rounded-full ${stringify(badgeClass)} px-2 py-1 text-xs font-medium`)}>${escape_html(formatConfidence(confidenceScore))} ${escape_html(alert.highestConfidence.toUpperCase())}</span></div></div></div> <div class="mb-4 space-y-3"><h4 class="text-sm font-medium text-gray-700">Matched Records (${escape_html(alert.totalMatchCount)}):</h4> <!--[-->`);
          const each_array_2 = ensure_array_like(alert.matches);
          for (let idx = 0, $$length2 = each_array_2.length; idx < $$length2; idx++) {
            let match = each_array_2[idx];
            $$renderer2.push(`<div class="rounded-lg border border-gray-200 bg-gray-50 p-3"><div class="flex items-start justify-between"><div class="flex-1"><div class="flex items-center gap-2"><span class="font-medium text-gray-900">${escape_html(match.recordName || match.recordId)}</span> <span class="text-xs text-gray-500">Match: ${escape_html(formatConfidence(match.matchResult?.score || 0))}</span></div> `);
            if (match.matchResult?.matchingFields?.length > 0) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<div class="mt-1 flex flex-wrap gap-1"><!--[-->`);
              const each_array_3 = ensure_array_like(match.matchResult?.matchingFields || []);
              for (let $$index_1 = 0, $$length3 = each_array_3.length; $$index_1 < $$length3; $$index_1++) {
                let field = each_array_3[$$index_1];
                $$renderer2.push(`<span class="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700">${escape_html(field)}</span>`);
              }
              $$renderer2.push(`<!--]--></div>`);
            } else {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]--></div></div></div>`);
          }
          $$renderer2.push(`<!--]--></div> <div class="flex gap-2"><button class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Dismiss</button> <button class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">Quick Merge</button> <button class="rounded-lg border border-blue-600 bg-white px-4 py-2 text-sm font-medium text-blue-600 hover:bg-blue-50">Merge</button></div></div></div></div>`);
        }
        $$renderer2.push(`<!--]--></div>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div> `);
    if (showBulkBar) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="fixed bottom-0 left-0 right-0 border-t border-gray-200 bg-white px-6 py-4 shadow-lg"><div class="mx-auto flex max-w-7xl items-center justify-between"><div class="flex items-center gap-4"><span class="text-sm font-medium text-gray-900">${escape_html(selectedIds.size)} selected</span> `);
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--></div> <div class="flex gap-2"><button${attr("disabled", bulkProcessing, true)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50">Dismiss All</button> <button${attr("disabled", bulkProcessing, true)} class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50">Merge All</button></div></div></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
