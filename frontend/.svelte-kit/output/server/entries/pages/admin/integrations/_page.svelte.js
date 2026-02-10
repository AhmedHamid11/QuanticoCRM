import "clsx";
import "../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><h1 class="text-2xl font-bold text-gray-900">Integrations</h1> <a href="/admin" class="text-sm text-blue-600 hover:text-blue-800">← Back to Admin</a></div> <p class="text-sm text-gray-600">Connect external systems to sync data and automate workflows.</p> <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6"><a href="/admin/integrations/salesforce" class="bg-white shadow rounded-lg p-6 hover:shadow-md transition-shadow border-l-4 border-blue-500"><div class="flex items-start"><div class="flex-shrink-0"><div class="h-12 w-12 bg-blue-500 rounded-lg flex items-center justify-center"><svg class="h-8 w-8 text-white" fill="currentColor" viewBox="0 0 24 24"><path d="M11.5 3a5.5 5.5 0 0 1 4.9 3h.1a4.5 4.5 0 0 1 0 9h-10a4.5 4.5 0 0 1-.4-9 5.5 5.5 0 0 1 5.4-3z"></path></svg></div></div> <div class="ml-4 flex-1"><div class="flex items-center justify-between"><h3 class="text-lg font-medium text-gray-900">Salesforce</h3> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> <p class="mt-1 text-sm text-gray-500">Sync merge instructions to Salesforce for seamless data integration</p> <div class="mt-3 text-sm text-blue-600 font-medium">Configure →</div></div></div></a> <div class="bg-white shadow rounded-lg p-6 border-l-4 border-gray-300 opacity-50"><div class="flex items-start"><div class="flex-shrink-0"><div class="h-12 w-12 bg-gray-300 rounded-lg flex items-center justify-center"><svg class="h-8 w-8 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg></div></div> <div class="ml-4"><h3 class="text-lg font-medium text-gray-500">More integrations coming soon</h3> <p class="mt-1 text-sm text-gray-400">Additional integrations will be available in future releases</p></div></div></div></div></div>`);
  });
}
export {
  _page as default
};
