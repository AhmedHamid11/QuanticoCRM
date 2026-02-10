import { X as ensure_array_like, Y as attr_class } from "./index.js";
import { g as getToasts } from "./toast.svelte.js";
import { e as escape_html } from "./attributes.js";
function Toast($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let toasts = getToasts();
    if (toasts.length > 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2"><!--[-->`);
      const each_array = ensure_array_like(toasts);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let toast = each_array[$$index];
        $$renderer2.push(`<div${attr_class("rounded-lg px-4 py-3 shadow-lg transition-all duration-300", void 0, {
          "bg-green-500": toast.type === "success",
          "bg-red-500": toast.type === "error",
          "bg-blue-600": toast.type === "info"
        })}><p class="text-white text-sm font-medium">${escape_html(toast.message)}</p></div>`);
      }
      $$renderer2.push(`<!--]--></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  Toast as T
};
