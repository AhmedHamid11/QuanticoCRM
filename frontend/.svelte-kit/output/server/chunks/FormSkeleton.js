import { Y as attr_class, X as ensure_array_like, Z as stringify } from "./index.js";
import { S as Skeleton } from "./Skeleton.js";
function FormSkeleton($$renderer, $$props) {
  let { fields = 6, showHeader = true, class: className = "" } = $$props;
  $$renderer.push(`<div${attr_class(`max-w-2xl mx-auto ${stringify(className)}`)}>`);
  if (showHeader) {
    $$renderer.push("<!--[-->");
    $$renderer.push(`<div class="flex items-center justify-between mb-6">`);
    Skeleton($$renderer, { variant: "heading", width: "10rem", height: "1.75rem" });
    $$renderer.push(`<!----> `);
    Skeleton($$renderer, { variant: "text", width: "8rem", height: "0.875rem" });
    $$renderer.push(`<!----></div>`);
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--> <div class="bg-white shadow rounded-lg p-6 space-y-4"><!--[-->`);
  const each_array = ensure_array_like(Array(fields));
  for (let i = 0, $$length = each_array.length; i < $$length; i++) {
    each_array[i];
    $$renderer.push(`<div>`);
    Skeleton($$renderer, {
      variant: "text",
      width: "30%",
      height: "0.875rem",
      class: "mb-1.5"
    });
    $$renderer.push(`<!----> `);
    Skeleton($$renderer, { variant: "input" });
    $$renderer.push(`<!----></div>`);
  }
  $$renderer.push(`<!--]--> <div class="flex justify-end gap-3 pt-4 border-t border-gray-200">`);
  Skeleton($$renderer, { variant: "button", width: "5rem" });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "button", width: "7rem" });
  $$renderer.push(`<!----></div></div></div>`);
}
export {
  FormSkeleton as F
};
