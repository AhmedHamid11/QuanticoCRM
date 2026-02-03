import "clsx";
import "../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div><div class="flex items-center space-x-4"><a href="/admin" class="text-gray-500 hover:text-gray-700 transition-colors"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"></path></svg></a> <div><h1 class="text-2xl font-bold text-gray-900">Changelog</h1> <p class="mt-1 text-sm text-gray-500">Platform changes and updates by version</p></div></div></div></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
      {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="bg-white shadow rounded-lg p-6"><div class="animate-pulse flex items-center space-x-4"><div class="h-4 bg-gray-200 rounded w-48"></div></div></div>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--> <div class="bg-white shadow rounded-lg overflow-hidden">`);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="px-6 py-12 text-center"><svg class="animate-spin mx-auto h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> <p class="mt-4 text-gray-500">Loading changelog...</p></div>`);
    }
    $$renderer2.push(`<!--]--></div></div>`);
  });
}
export {
  _page as default
};
