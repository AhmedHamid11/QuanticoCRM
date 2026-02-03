import { Y as attr_class, Z as stringify, X as ensure_array_like } from "../../../chunks/index.js";
import { g as get } from "../../../chunks/api.js";
import { t as toast } from "../../../chunks/toast.svelte.js";
import { T as TableSkeleton } from "../../../chunks/TableSkeleton.js";
import { B as Button } from "../../../chunks/Button.js";
import { e as escape_html, a as attr } from "../../../chunks/attributes.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../chunks/state.svelte.js";
function ErrorDisplay($$renderer, $$props) {
  let {
    message = "Something went wrong. Please try again.",
    title = "Error",
    onRetry,
    showHomeLink = false,
    class: className = ""
  } = $$props;
  $$renderer.push(`<div${attr_class(`text-center py-12 px-4 ${stringify(className)}`)}><div class="mx-auto w-16 h-16 rounded-full bg-red-100 flex items-center justify-center mb-4"><svg class="w-8 h-8 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg></div> <h3 class="text-lg font-medium text-gray-900 mb-2">${escape_html(title)}</h3> <p class="text-gray-500 mb-6 max-w-md mx-auto">${escape_html(message)}</p> <div class="flex justify-center gap-3">`);
  if (onRetry) {
    $$renderer.push("<!--[-->");
    Button($$renderer, {
      variant: "primary",
      onclick: onRetry,
      children: ($$renderer2) => {
        $$renderer2.push(`<!---->Try Again`);
      }
    });
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--> `);
  if (showHomeLink) {
    $$renderer.push("<!--[-->");
    Button($$renderer, {
      variant: "secondary",
      onclick: () => window.location.href = "/",
      children: ($$renderer2) => {
        $$renderer2.push(`<!---->Go Home`);
      }
    });
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--></div></div>`);
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let tasks = [];
    let loading = true;
    let error = null;
    let search = "";
    let page = 1;
    let pageSize = 20;
    let total = 0;
    let totalPages = 0;
    let sortBy = "created_at";
    let sortDir = "desc";
    let statusFilter = "";
    let typeFilter = "";
    let knownTotal = null;
    let filterQuery = "";
    let showFilterPanel = false;
    let listViews = [];
    let selectedListView = null;
    async function loadTasks() {
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
        if (statusFilter) ;
        if (typeFilter) ;
        if (filterQuery.trim()) {
          params.set("filter", filterQuery.trim());
        }
        if (knownTotal !== null && page > 1) {
          params.set("knownTotal", knownTotal.toString());
        }
        const result = await get(`/tasks?${params}`);
        tasks = result.data;
        total = result.total;
        totalPages = result.totalPages;
        knownTotal = result.total;
      } catch (e) {
        error = e instanceof Error ? e.message : "Failed to load tasks";
        toast.error(error);
      } finally {
        loading = false;
      }
    }
    function selectListView(view) {
      selectedListView = view;
      if (view) {
        filterQuery = view.filterQuery || "";
        sortBy = view.sortBy || "created_at";
        sortDir = view.sortDir || "desc";
        showFilterPanel = !!view.filterQuery;
      } else {
        filterQuery = "";
        sortBy = "created_at";
        sortDir = "desc";
      }
      page = 1;
      knownTotal = null;
      loadTasks();
    }
    function handleFilterChange() {
      page = 1;
      knownTotal = null;
      loadTasks();
    }
    function formatDate(dateStr) {
      if (!dateStr) return "-";
      return new Date(dateStr).toLocaleDateString();
    }
    function getStatusColor(status) {
      switch (status) {
        case "Open":
          return "bg-blue-100 text-blue-800";
        case "In Progress":
          return "bg-yellow-100 text-yellow-800";
        case "Completed":
          return "bg-green-100 text-green-800";
        case "Deferred":
          return "bg-gray-100 text-gray-800";
        case "Cancelled":
          return "bg-red-100 text-red-800";
        default:
          return "bg-gray-100 text-gray-800";
      }
    }
    function getPriorityColor(priority) {
      switch (priority) {
        case "Urgent":
          return "text-red-600 font-semibold";
        case "High":
          return "text-orange-600";
        case "Normal":
          return "text-gray-600";
        case "Low":
          return "text-gray-400";
        default:
          return "text-gray-600";
      }
    }
    $$renderer2.push(`<div class="space-y-4"><div class="flex justify-between items-center"><h1 class="text-2xl font-bold text-gray-900">Tasks</h1> <a href="/tasks/new" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700">+ New Task</a></div> <div class="flex gap-4 flex-wrap items-center"><div class="flex-1 min-w-64 relative"><input type="text"${attr("value", search)} placeholder="Search tasks..." class="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/> <svg class="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clip-rule="evenodd"></path></svg></div> `);
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
        $$renderer3.option({ value: "Open" }, ($$renderer4) => {
          $$renderer4.push(`Open`);
        });
        $$renderer3.option({ value: "In Progress" }, ($$renderer4) => {
          $$renderer4.push(`In Progress`);
        });
        $$renderer3.option({ value: "Completed" }, ($$renderer4) => {
          $$renderer4.push(`Completed`);
        });
        $$renderer3.option({ value: "Deferred" }, ($$renderer4) => {
          $$renderer4.push(`Deferred`);
        });
        $$renderer3.option({ value: "Cancelled" }, ($$renderer4) => {
          $$renderer4.push(`Cancelled`);
        });
      }
    );
    $$renderer2.push(` `);
    $$renderer2.select(
      {
        value: typeFilter,
        onchange: handleFilterChange,
        class: "px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Types`);
        });
        $$renderer3.option({ value: "Call" }, ($$renderer4) => {
          $$renderer4.push(`Call`);
        });
        $$renderer3.option({ value: "Email" }, ($$renderer4) => {
          $$renderer4.push(`Email`);
        });
        $$renderer3.option({ value: "Meeting" }, ($$renderer4) => {
          $$renderer4.push(`Meeting`);
        });
        $$renderer3.option({ value: "Todo" }, ($$renderer4) => {
          $$renderer4.push(`Todo`);
        });
      }
    );
    $$renderer2.push(` <button${attr_class(`px-3 py-2 border rounded-md text-sm font-medium ${stringify(showFilterPanel || filterQuery ? "border-blue-500 bg-blue-50 text-blue-700" : "border-gray-300 text-gray-700 hover:bg-gray-50")}`)} aria-label="Toggle filter panel"><svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"></path></svg> Filter `);
    if (filterQuery) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<span class="ml-1 text-xs bg-blue-500 text-white px-1.5 py-0.5 rounded-full">1</span>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></button> `);
    if (listViews.length > 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="relative">`);
      $$renderer2.select(
        {
          value: selectedListView,
          onchange: () => selectListView(selectedListView),
          class: "px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500 pr-8"
        },
        ($$renderer3) => {
          $$renderer3.option({ value: null }, ($$renderer4) => {
            $$renderer4.push(`All Tasks`);
          });
          $$renderer3.push(`<!--[-->`);
          const each_array = ensure_array_like(listViews);
          for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
            let view = each_array[$$index];
            $$renderer3.option({ value: view }, ($$renderer4) => {
              $$renderer4.push(`${escape_html(view.name)} ${escape_html(view.isDefault ? "(Default)" : "")}`);
            });
          }
          $$renderer3.push(`<!--]-->`);
        }
      );
      $$renderer2.push(`</div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    if (listViews.length > 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<button class="px-3 py-2 text-sm text-gray-600 hover:text-gray-900" aria-label="Manage saved views">Manage Views</button>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    if (showFilterPanel) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3"><div class="flex items-start gap-4"><div class="flex-1"><label for="filter-query" class="block text-sm font-medium text-gray-700 mb-1">Filter Query (SQL-style WHERE clause)</label> <textarea id="filter-query" placeholder="e.g., status = 'Open' AND priority = 'High'" rows="2" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500 font-mono text-sm">`);
      const $$body = escape_html(filterQuery);
      if ($$body) {
        $$renderer2.push(`${$$body}`);
      }
      $$renderer2.push(`</textarea></div> <div class="flex flex-col gap-2 pt-6"><button class="px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-md hover:bg-green-700">Save View</button> <button class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700">Apply</button> <button class="px-4 py-2 border border-gray-300 text-sm font-medium rounded-md hover:bg-gray-50">Clear</button></div></div> <details class="text-sm text-gray-600"><summary class="cursor-pointer hover:text-gray-900">Filter Syntax Help</summary> <div class="mt-2 pl-4 space-y-1"><p><code class="bg-gray-200 px-1 rounded">status = 'Open'</code> - Exact match</p> <p><code class="bg-gray-200 px-1 rounded">subject LIKE '%report%'</code> - Contains</p> <p><code class="bg-gray-200 px-1 rounded">priority IN ('High', 'Urgent')</code> - Multiple values</p> <p><code class="bg-gray-200 px-1 rounded">due_date > TODAY</code> - Future tasks</p> <p><code class="bg-gray-200 px-1 rounded">due_date &lt; TODAY</code> - Overdue tasks</p> <p><code class="bg-gray-200 px-1 rounded">due_date &lt;= TODAY + 7</code> - Due within a week</p> <p><code class="bg-gray-200 px-1 rounded">due_date > TODAY - 1m</code> - Last month (d=days, w=weeks, m=months)</p> <p><code class="bg-gray-200 px-1 rounded">status = 'Open' AND priority = 'High'</code> - Combined</p></div></details></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    if (loading) {
      $$renderer2.push("<!--[-->");
      TableSkeleton($$renderer2, { rows: pageSize, columns: 7 });
    } else {
      $$renderer2.push("<!--[!-->");
      if (error) {
        $$renderer2.push("<!--[-->");
        ErrorDisplay($$renderer2, { message: error, onRetry: loadTasks });
      } else {
        $$renderer2.push("<!--[!-->");
        if (tasks.length === 0) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="text-center py-12 text-gray-500">No tasks found. <a href="/tasks/new" class="text-blue-600 hover:underline">Create one</a></div>`);
        } else {
          $$renderer2.push("<!--[!-->");
          $$renderer2.push(`<div class="bg-white shadow rounded-lg overflow-hidden"><table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Subject `);
          if (sortBy === "subject") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Status `);
          if (sortBy === "status") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Type `);
          if (sortBy === "type") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Priority `);
          if (sortBy === "priority") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Due Date `);
          if (sortBy === "due_date") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Related To</th><th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100">Last Modified `);
          if (sortBy === "modified_at") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="ml-1">${escape_html(sortDir === "asc" ? "↑" : "↓")}</span>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></th><th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
          const each_array_1 = ensure_array_like(tasks);
          for (let $$index_1 = 0, $$length = each_array_1.length; $$index_1 < $$length; $$index_1++) {
            let task = each_array_1[$$index_1];
            $$renderer2.push(`<tr class="hover:bg-gray-50"><td class="px-6 py-4 whitespace-nowrap"><a${attr("href", `/tasks/${stringify(task.id)}`)} class="text-blue-600 hover:underline font-medium">${escape_html(task.subject)}</a></td><td class="px-6 py-4 whitespace-nowrap"><span${attr_class(`px-2 py-1 text-xs font-medium rounded-full ${stringify(getStatusColor(task.status))}`)}>${escape_html(task.status)}</span></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(task.type)}</td><td${attr_class(`px-6 py-4 whitespace-nowrap text-sm ${stringify(getPriorityColor(task.priority))}`)}>${escape_html(task.priority)}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(formatDate(task.dueDate))}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">`);
            if (task.parentType && task.parentId) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<a${attr("href", `/${stringify(task.parentType.toLowerCase())}s/${stringify(task.parentId)}`)} class="text-blue-600 hover:underline">${escape_html(task.parentName || task.parentType)}</a>`);
            } else {
              $$renderer2.push("<!--[!-->");
              $$renderer2.push(`-`);
            }
            $$renderer2.push(`<!--]--></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500"><div>${escape_html(formatDate(task.modifiedAt))}</div> `);
            if (task.modifiedByName) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<div class="text-xs text-gray-400">${escape_html(task.modifiedByName)}</div>`);
            } else {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]--></td><td class="px-6 py-4 whitespace-nowrap text-right text-sm"><a${attr("href", `/tasks/${stringify(task.id)}/edit`)} class="text-blue-600 hover:underline mr-4">Edit</a> <button class="text-red-600 hover:underline">Delete</button></td></tr>`);
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
