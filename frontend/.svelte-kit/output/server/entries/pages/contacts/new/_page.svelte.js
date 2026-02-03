import "clsx";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import { F as FormSkeleton } from "../../../../chunks/FormSkeleton.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    $$renderer2.push(`<div class="max-w-2xl mx-auto"><div class="flex items-center justify-between mb-6"><h1 class="text-2xl font-bold text-gray-900">New Contact</h1> <a href="/contacts" class="text-gray-600 hover:text-gray-900 text-sm">← Back to Contacts</a></div> `);
    {
      $$renderer2.push("<!--[-->");
      FormSkeleton($$renderer2, { fields: 6 });
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
