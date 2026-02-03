import "clsx";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="space-y-6"><div class="flex justify-between items-center"><div><h1 class="text-2xl font-bold text-gray-900">Navigation</h1> <p class="mt-1 text-sm text-gray-500">Configure the tabs that appear in the toolbar</p></div> <button class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors">Add Tab</button></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12"><div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div> <p class="mt-2 text-gray-500">Loading...</p></div>`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
