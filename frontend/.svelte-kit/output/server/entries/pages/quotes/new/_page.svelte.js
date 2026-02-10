import { X as ensure_array_like, Y as attr_class, Z as stringify, a2 as bind_props } from "../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { e as escape_html, a as attr } from "../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import "../../../../chunks/auth.svelte.js";
import { L as LookupField } from "../../../../chunks/LookupField.js";
const QUOTE_STATUSES = [
  "Draft",
  "Needs Review",
  "Approved",
  "Sent",
  "Accepted",
  "Declined",
  "Expired"
];
function formatCurrency(amount, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2
  }).format(amount);
}
function QuoteLineItemEditor($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      items = [],
      fields = [],
      currency = "USD",
      disabled = false,
      onchange
    } = $$props;
    const defaultVisibleFields = [
      "name",
      "description",
      "sku",
      "quantity",
      "unitPrice",
      "discountPercent",
      "total"
    ];
    let visibleFields = fields.length > 0 ? fields.filter((f) => !["quoteId", "sortOrder", "createdAt", "modifiedAt", "id"].includes(f.name)).sort((a, b) => a.sortOrder - b.sortOrder) : defaultVisibleFields.map((name, i) => ({
      id: name,
      entityName: "QuoteLineItem",
      name,
      label: getDefaultLabel(name),
      type: getDefaultType(name),
      isRequired: ["name", "quantity", "unitPrice"].includes(name),
      isReadOnly: name === "total",
      isAudited: false,
      isCustom: false,
      sortOrder: i,
      createdAt: "",
      modifiedAt: ""
    }));
    function getDefaultLabel(name) {
      const labels = {
        name: "Item",
        description: "Description",
        sku: "SKU",
        quantity: "Qty",
        unitPrice: "Price",
        discountPercent: "Disc %",
        discountAmount: "Disc Amt",
        taxPercent: "Tax %",
        total: "Total"
      };
      return labels[name] || name;
    }
    function getDefaultType(name) {
      const types = {
        name: "varchar",
        description: "text",
        sku: "varchar",
        quantity: "float",
        unitPrice: "currency",
        discountPercent: "float",
        discountAmount: "currency",
        taxPercent: "float",
        total: "currency"
      };
      return types[name] || "varchar";
    }
    function calcLineTotal(item) {
      let total = item.quantity * item.unitPrice;
      if (item.discountPercent > 0) {
        total -= total * item.discountPercent / 100;
      } else if (item.discountAmount > 0) {
        total -= item.discountAmount;
      }
      return Math.round(total * 100) / 100;
    }
    function getFieldValue(item, fieldName) {
      if (fieldName === "total") {
        return calcLineTotal(item);
      }
      return item[fieldName];
    }
    function getColumnWidth(fieldName) {
      const widths = {
        name: "",
        description: "",
        sku: "w-20",
        quantity: "w-16",
        unitPrice: "w-24",
        discountPercent: "w-20",
        discountAmount: "w-24",
        taxPercent: "w-20",
        total: "w-24"
      };
      return widths[fieldName] || "w-24";
    }
    function getColumnAlign(field) {
      if (["float", "int", "currency"].includes(field.type)) {
        return "text-right";
      }
      return "text-left";
    }
    let subtotal = items.reduce((sum, item) => sum + calcLineTotal(item), 0);
    let columnCount = visibleFields.length + 1 + (disabled ? 0 : 1);
    $$renderer2.push(`<div class="space-y-3"><div class="flex items-center justify-between"><h3 class="text-sm font-medium text-gray-700">Line Items</h3> `);
    if (!disabled) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<button type="button" class="inline-flex items-center px-3 py-1.5 text-xs font-medium text-blue-700 bg-blue-50 rounded-md hover:bg-blue-100"><svg class="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg> Add Item</button>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    if (items.length === 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-8 text-gray-400 border-2 border-dashed border-gray-200 rounded-lg"><p class="text-sm">No line items yet</p> `);
      if (!disabled) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<button type="button" class="mt-2 text-sm text-blue-600 hover:text-blue-700">Add your first item</button>`);
      } else {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<div class="overflow-x-auto border border-gray-200 rounded-lg"><table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th class="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase w-8">#</th><!--[-->`);
      const each_array = ensure_array_like(visibleFields);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let field = each_array[$$index];
        $$renderer2.push(`<th${attr_class(`px-3 py-2 ${stringify(getColumnAlign(field))} text-xs font-medium text-gray-500 uppercase ${stringify(getColumnWidth(field.name))}`)}>${escape_html(field.label)}</th>`);
      }
      $$renderer2.push(`<!--]-->`);
      if (!disabled) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<th class="px-3 py-2 w-20"></th>`);
      } else {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
      const each_array_1 = ensure_array_like(items);
      for (let i = 0, $$length = each_array_1.length; i < $$length; i++) {
        let item = each_array_1[i];
        $$renderer2.push(`<tr><td class="px-3 py-2 text-sm text-gray-500">${escape_html(i + 1)}</td><!--[-->`);
        const each_array_2 = ensure_array_like(visibleFields);
        for (let $$index_1 = 0, $$length2 = each_array_2.length; $$index_1 < $$length2; $$index_1++) {
          let field = each_array_2[$$index_1];
          $$renderer2.push(`<td${attr_class(`px-3 py-2 ${stringify(getColumnAlign(field))}`)}>`);
          if (field.isReadOnly || field.name === "total") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<span class="text-sm font-medium">`);
            if (field.type === "currency") {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`${escape_html(formatCurrency(getFieldValue(item, field.name), currency))}`);
            } else {
              $$renderer2.push("<!--[!-->");
              $$renderer2.push(`${escape_html(getFieldValue(item, field.name) ?? "-")}`);
            }
            $$renderer2.push(`<!--]--></span>`);
          } else {
            $$renderer2.push("<!--[!-->");
            if (field.name === "name") {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<input type="text"${attr("value", item.name)} placeholder="Item name"${attr("disabled", disabled, true)} class="w-full text-sm border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"/> `);
              if (visibleFields.some((f) => f.name === "description")) {
                $$renderer2.push("<!--[-->");
              } else {
                $$renderer2.push("<!--[!-->");
              }
              $$renderer2.push(`<!--]-->`);
            } else {
              $$renderer2.push("<!--[!-->");
              if (field.name === "description") {
                $$renderer2.push("<!--[-->");
                $$renderer2.push(`<input type="text"${attr("value", item.description ?? "")} placeholder="-"${attr("disabled", disabled, true)} class="w-full text-xs text-gray-400 border-0 focus:ring-0 p-0 bg-transparent"/>`);
              } else {
                $$renderer2.push("<!--[!-->");
                if (field.type === "float" || field.type === "int") {
                  $$renderer2.push("<!--[-->");
                  $$renderer2.push(`<input type="number"${attr("value", getFieldValue(item, field.name) ?? 0)}${attr("min", field.minValue ?? 0)}${attr("max", field.maxValue ?? void 0)}${attr("step", field.type === "int" ? "1" : "0.01")}${attr("disabled", disabled, true)} class="w-full text-sm text-right border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"/>`);
                } else {
                  $$renderer2.push("<!--[!-->");
                  if (field.type === "currency") {
                    $$renderer2.push("<!--[-->");
                    $$renderer2.push(`<input type="number"${attr("value", getFieldValue(item, field.name) ?? 0)} min="0" step="0.01"${attr("disabled", disabled, true)} class="w-full text-sm text-right border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"/>`);
                  } else {
                    $$renderer2.push("<!--[!-->");
                    $$renderer2.push(`<input type="text"${attr("value", getFieldValue(item, field.name) ?? "")} placeholder="-"${attr("disabled", disabled, true)} class="w-full text-sm border-0 border-b border-transparent focus:border-blue-500 focus:ring-0 p-0 bg-transparent"/>`);
                  }
                  $$renderer2.push(`<!--]-->`);
                }
                $$renderer2.push(`<!--]-->`);
              }
              $$renderer2.push(`<!--]-->`);
            }
            $$renderer2.push(`<!--]-->`);
          }
          $$renderer2.push(`<!--]--></td>`);
        }
        $$renderer2.push(`<!--]-->`);
        if (!disabled) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<td class="px-3 py-2"><div class="flex items-center gap-1"><button type="button"${attr("disabled", i === 0, true)} class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30" title="Move up"><svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7"></path></svg></button> <button type="button"${attr("disabled", i === items.length - 1, true)} class="p-1 text-gray-400 hover:text-gray-600 disabled:opacity-30" title="Move down"><svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg></button> <button type="button" class="p-1 text-red-400 hover:text-red-600" title="Remove"><svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg></button></div></td>`);
        } else {
          $$renderer2.push("<!--[!-->");
        }
        $$renderer2.push(`<!--]--></tr>`);
      }
      $$renderer2.push(`<!--]--></tbody><tfoot><tr class="bg-gray-50"><td${attr("colspan", columnCount - 1)} class="px-3 py-2 text-right text-sm font-medium text-gray-700">Subtotal</td><td class="px-3 py-2 text-right text-sm font-bold">${escape_html(formatCurrency(subtotal, currency))}</td></tr></tfoot></table></div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
    bind_props($$props, { items });
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let saving = false;
    let lineItemFields = [];
    let name = "";
    let status = "Draft";
    let accountId = null;
    let accountName = "";
    let contactId = null;
    let contactName = "";
    let validUntil = "";
    let currency = "USD";
    let discountPercent = 0;
    let taxPercent = 0;
    let shippingAmount = 0;
    let description = "";
    let terms = "";
    let notes = "";
    let lineItems = [];
    let $$settled = true;
    let $$inner_renderer;
    function $$render_inner($$renderer3) {
      $$renderer3.push(`<div class="max-w-4xl mx-auto"><div class="flex items-center justify-between mb-6"><h1 class="text-2xl font-bold text-gray-900">New Quote</h1> <a href="/quotes" class="text-gray-600 hover:text-gray-900 text-sm">← Back to Quotes</a></div> <form class="space-y-6"><div class="bg-white shadow rounded-lg p-6 space-y-4"><h2 class="text-lg font-medium text-gray-900">Quote Details</h2> <div class="grid grid-cols-1 md:grid-cols-2 gap-4"><div><label for="name" class="block text-sm font-medium text-gray-700 mb-1">Name <span class="text-red-500">*</span></label> <input id="name" type="text"${attr("value", name)} required class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="status" class="block text-sm font-medium text-gray-700 mb-1">Status</label> `);
      $$renderer3.select(
        {
          id: "status",
          value: status,
          class: "w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
        },
        ($$renderer4) => {
          $$renderer4.push(`<!--[-->`);
          const each_array = ensure_array_like(QUOTE_STATUSES);
          for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
            let s = each_array[$$index];
            $$renderer4.option({ value: s }, ($$renderer5) => {
              $$renderer5.push(`${escape_html(s)}`);
            });
          }
          $$renderer4.push(`<!--]-->`);
        }
      );
      $$renderer3.push(`</div> <div>`);
      LookupField($$renderer3, {
        entity: "Account",
        value: accountId,
        valueName: accountName,
        label: "Account",
        onchange: (id, n) => {
          accountId = id;
          accountName = n;
        }
      });
      $$renderer3.push(`<!----></div> <div>`);
      LookupField($$renderer3, {
        entity: "Contact",
        value: contactId,
        valueName: contactName,
        label: "Contact",
        onchange: (id, n) => {
          contactId = id;
          contactName = n;
        }
      });
      $$renderer3.push(`<!----></div> <div><label for="validUntil" class="block text-sm font-medium text-gray-700 mb-1">Valid Until</label> <input id="validUntil" type="date"${attr("value", validUntil)} class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="currency" class="block text-sm font-medium text-gray-700 mb-1">Currency</label> `);
      $$renderer3.select(
        {
          id: "currency",
          value: currency,
          class: "w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
        },
        ($$renderer4) => {
          $$renderer4.option({ value: "USD" }, ($$renderer5) => {
            $$renderer5.push(`USD`);
          });
          $$renderer4.option({ value: "EUR" }, ($$renderer5) => {
            $$renderer5.push(`EUR`);
          });
          $$renderer4.option({ value: "GBP" }, ($$renderer5) => {
            $$renderer5.push(`GBP`);
          });
        }
      );
      $$renderer3.push(`</div></div></div> <div class="bg-white shadow rounded-lg p-6">`);
      QuoteLineItemEditor($$renderer3, {
        fields: lineItemFields,
        currency,
        get items() {
          return lineItems;
        },
        set items($$value) {
          lineItems = $$value;
          $$settled = false;
        }
      });
      $$renderer3.push(`<!----></div> <div class="bg-white shadow rounded-lg p-6 space-y-4"><h2 class="text-lg font-medium text-gray-900">Pricing</h2> <div class="grid grid-cols-1 md:grid-cols-3 gap-4"><div><label for="discountPercent" class="block text-sm font-medium text-gray-700 mb-1">Discount %</label> <input id="discountPercent" type="number"${attr("value", discountPercent)} min="0" max="100" step="0.1" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="taxPercent" class="block text-sm font-medium text-gray-700 mb-1">Tax %</label> <input id="taxPercent" type="number"${attr("value", taxPercent)} min="0" step="0.1" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="shippingAmount" class="block text-sm font-medium text-gray-700 mb-1">Shipping</label> <input id="shippingAmount" type="number"${attr("value", shippingAmount)} min="0" step="0.01" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div></div></div> <div class="bg-white shadow rounded-lg p-6 space-y-4"><h2 class="text-lg font-medium text-gray-900">Additional Information</h2> <div><label for="description" class="block text-sm font-medium text-gray-700 mb-1">Description</label> <textarea id="description" rows="3" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">`);
      const $$body = escape_html(description);
      if ($$body) {
        $$renderer3.push(`${$$body}`);
      }
      $$renderer3.push(`</textarea></div> <div><label for="terms" class="block text-sm font-medium text-gray-700 mb-1">Terms &amp; Conditions</label> <textarea id="terms" rows="3" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">`);
      const $$body_1 = escape_html(terms);
      if ($$body_1) {
        $$renderer3.push(`${$$body_1}`);
      }
      $$renderer3.push(`</textarea></div> <div><label for="notes" class="block text-sm font-medium text-gray-700 mb-1">Notes</label> <textarea id="notes" rows="2" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">`);
      const $$body_2 = escape_html(notes);
      if ($$body_2) {
        $$renderer3.push(`${$$body_2}`);
      }
      $$renderer3.push(`</textarea></div></div> <div class="flex justify-end gap-3"><a href="/quotes" class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">Cancel</a> <button type="submit"${attr("disabled", saving, true)} class="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50">${escape_html("Create Quote")}</button></div></form></div>`);
    }
    do {
      $$settled = true;
      $$inner_renderer = $$renderer2.copy();
      $$render_inner($$inner_renderer);
    } while (!$$settled);
    $$renderer2.subsume($$inner_renderer);
  });
}
export {
  _page as default
};
