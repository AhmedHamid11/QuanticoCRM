import "clsx";
import "../../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="max-w-2xl mx-auto space-y-6"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Webhook Settings</h1> <p class="text-sm text-gray-500 mt-1">Configure authentication and timeout settings for outgoing webhooks</p></div> <a href="/admin/tripwires" class="text-sm text-gray-600 hover:text-gray-900">← Back to Tripwires</a></div> `);
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
