import "clsx";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import "../../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><h1 class="text-2xl font-bold text-gray-900">Salesforce Integration</h1> <a href="/admin/integrations" class="text-sm text-blue-600 hover:text-blue-800">← Back to Integrations</a></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-white shadow rounded-lg p-6"><div class="animate-pulse space-y-4"><div class="h-4 bg-gray-200 rounded w-1/4"></div> <div class="h-10 bg-gray-200 rounded w-1/2"></div></div></div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
