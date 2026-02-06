import "clsx";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import "../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let fields = [];
    fields.filter((f) => f.name !== "id" && !f.isReadOnly && f.type !== "rollup");
    $$renderer2.push(`<div class="space-y-6"><div class="flex justify-between items-center"><div><div class="flex items-center space-x-2 text-sm text-gray-500 mb-2"><a href="/accounts" class="hover:text-gray-700">Accounts</a> <span>/</span> <span>New</span></div> <h1 class="text-2xl font-bold text-gray-900">New Account</h1></div></div> `);
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
