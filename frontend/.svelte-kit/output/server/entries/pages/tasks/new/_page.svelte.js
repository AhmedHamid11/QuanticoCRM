import { W as store_get, _ as unsubscribe_stores, Z as stringify } from "../../../../chunks/index.js";
import { p as page } from "../../../../chunks/stores.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { a as attr, e as escape_html } from "../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import { L as LookupField } from "../../../../chunks/LookupField.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("parentType");
    store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("parentId");
    let returnUrl = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("returnUrl");
    const today = (/* @__PURE__ */ new Date()).toISOString().split("T")[0];
    let formData = {
      subject: "",
      description: "",
      status: "Open",
      priority: "Normal",
      type: "Todo",
      dueDate: today,
      parentId: null,
      parentType: null,
      parentName: ""
    };
    let parentName = "";
    let saving = false;
    function handleParentChange(entity) {
      return (id, name) => {
        formData.parentId = id;
        formData.parentType = id ? entity : null;
        formData.parentName = name;
        parentName = name;
      };
    }
    $$renderer2.push(`<div class="max-w-2xl mx-auto"><div class="mb-6"><nav class="text-sm text-gray-500 mb-2"><a href="/tasks" class="hover:text-gray-700">Tasks</a> <span class="mx-2">/</span> <span class="text-gray-900">New Task</span></nav> <h1 class="text-2xl font-bold text-gray-900">Create Task</h1></div> <form class="bg-white shadow rounded-lg p-6 space-y-6"><div><label for="subject" class="block text-sm font-medium text-gray-700 mb-1">Subject <span class="text-red-500">*</span></label> <input type="text" id="subject"${attr("value", formData.subject)} required class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" placeholder="Enter task subject"/></div> <div class="grid grid-cols-2 gap-4"><div><label for="type" class="block text-sm font-medium text-gray-700 mb-1">Type</label> `);
    $$renderer2.select(
      {
        id: "type",
        value: formData.type,
        class: "w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
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
    $$renderer2.push(`</div> <div><label for="status" class="block text-sm font-medium text-gray-700 mb-1">Status</label> `);
    $$renderer2.select(
      {
        id: "status",
        value: formData.status,
        class: "w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
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
    $$renderer2.push(`</div></div> <div class="grid grid-cols-2 gap-4"><div><label for="priority" class="block text-sm font-medium text-gray-700 mb-1">Priority</label> `);
    $$renderer2.select(
      {
        id: "priority",
        value: formData.priority,
        class: "w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "Low" }, ($$renderer4) => {
          $$renderer4.push(`Low`);
        });
        $$renderer3.option({ value: "Normal" }, ($$renderer4) => {
          $$renderer4.push(`Normal`);
        });
        $$renderer3.option({ value: "High" }, ($$renderer4) => {
          $$renderer4.push(`High`);
        });
        $$renderer3.option({ value: "Urgent" }, ($$renderer4) => {
          $$renderer4.push(`Urgent`);
        });
      }
    );
    $$renderer2.push(`</div> <div><label for="dueDate" class="block text-sm font-medium text-gray-700 mb-1">Due Date</label> <input type="date" id="dueDate"${attr("value", formData.dueDate)} class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div></div> <div><label class="block text-sm font-medium text-gray-700 mb-1">Related To</label> `);
    if (formData.parentId && formData.parentType) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="flex items-center gap-2 px-3 py-2 border border-gray-300 rounded-md bg-gray-50"><span class="text-xs text-gray-500 uppercase">${escape_html(formData.parentType)}:</span> <a${attr("href", `/${stringify(formData.parentType.toLowerCase())}s/${stringify(formData.parentId)}`)} class="text-blue-600 hover:underline flex-1">${escape_html(parentName || formData.parentName || "Loading...")}</a> <button type="button" class="text-gray-400 hover:text-gray-600" aria-label="Clear selection"><svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg></button></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<div class="grid grid-cols-2 gap-4">`);
      LookupField($$renderer2, {
        entity: "Account",
        value: formData.parentType === "Account" ? formData.parentId ?? null : null,
        valueName: formData.parentType === "Account" ? parentName : "",
        label: "Account",
        onchange: handleParentChange("Account")
      });
      $$renderer2.push(`<!----> `);
      LookupField($$renderer2, {
        entity: "Contact",
        value: formData.parentType === "Contact" ? formData.parentId ?? null : null,
        valueName: formData.parentType === "Contact" ? parentName : "",
        label: "Contact",
        onchange: handleParentChange("Contact")
      });
      $$renderer2.push(`<!----></div>`);
    }
    $$renderer2.push(`<!--]--></div> <div><label for="description" class="block text-sm font-medium text-gray-700 mb-1">Description</label> <textarea id="description" rows="4" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" placeholder="Enter description">`);
    const $$body = escape_html(formData.description);
    if ($$body) {
      $$renderer2.push(`${$$body}`);
    }
    $$renderer2.push(`</textarea></div> <div class="flex justify-end gap-3 pt-4 border-t"><a${attr("href", returnUrl || "/tasks")} class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50">Cancel</a> <button type="submit"${attr("disabled", saving, true)} class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50">${escape_html("Create Task")}</button></div></form></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
