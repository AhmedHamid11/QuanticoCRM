import { X as ensure_array_like } from "../../../../../chunks/index.js";
import "../../../../../chunks/auth.svelte.js";
import { e as escape_html } from "../../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let entityFilter = "";
    let entities = [];
    $$renderer2.push(`<div class="max-w-7xl mx-auto px-4 py-8"><div class="mb-8 flex items-center justify-between"><div><h1 class="text-3xl font-bold text-gray-900">Duplicate Rules</h1> <p class="mt-2 text-sm text-gray-600">Configure matching rules to detect duplicate records</p></div> <button class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors">New Rule</button></div> <div class="mb-6"><label for="rules-entity-filter" class="block text-sm font-medium text-gray-700 mb-2">Filter by Entity Type</label> `);
    $$renderer2.select(
      {
        id: "rules-entity-filter",
        name: "rules-entity-filter",
        value: entityFilter,
        class: "w-64 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
      },
      ($$renderer3) => {
        $$renderer3.option({ value: "" }, ($$renderer4) => {
          $$renderer4.push(`All Entities`);
        });
        $$renderer3.push(`<!--[-->`);
        const each_array_6 = ensure_array_like(entities);
        for (let $$index = 0, $$length = each_array_6.length; $$index < $$length; $$index++) {
          let entity = each_array_6[$$index];
          $$renderer3.option({ value: entity.name }, ($$renderer4) => {
            $$renderer4.push(`${escape_html(entity.label)}`);
          });
        }
        $$renderer3.push(`<!--]-->`);
      }
    );
    $$renderer2.push(`</div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading rules...</div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
