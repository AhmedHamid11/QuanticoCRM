import "clsx";
import "@sveltejs/kit/internal";
import "../../../../../chunks/exports.js";
import "../../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../../chunks/state.svelte.js";
import "../../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="max-w-6xl mx-auto space-y-6"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">New Screen Flow</h1> <p class="text-sm text-gray-500 mt-1">Create an interactive wizard with screens, decisions, and actions</p></div> <a href="/admin/flows" class="text-sm text-gray-600 hover:text-gray-900">← Back to Flows</a></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading...</div>`);
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
