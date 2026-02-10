import { a as attr } from "../../../chunks/attributes.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../chunks/state.svelte.js";
import "../../../chunks/auth.svelte.js";
import { T as TableSkeleton } from "../../../chunks/TableSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let search = "";
    $$renderer2.push(`<div class="space-y-4"><div class="flex items-center justify-between"><h1 class="text-2xl font-bold text-gray-900">Quotes</h1> <a href="/quotes/new" class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90"><svg class="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg> New Quote</a></div> <div class="flex gap-3"><div class="flex-1"><input type="text"${attr("value", search)} placeholder="Search quotes..." class="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"/></div></div> `);
    {
      $$renderer2.push("<!--[-->");
      TableSkeleton($$renderer2, {});
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
