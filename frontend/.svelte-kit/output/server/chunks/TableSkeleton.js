import { Y as attr_class, X as ensure_array_like, Z as stringify } from "./index.js";
import { S as Skeleton } from "./Skeleton.js";
function TableSkeleton($$renderer, $$props) {
  let {
    rows = 5,
    columns = 5,
    showHeader = true,
    class: className = ""
  } = $$props;
  $$renderer.push(`<div${attr_class(`bg-white shadow rounded-lg overflow-hidden ${stringify(className)}`)}><table class="min-w-full divide-y divide-gray-200">`);
  if (showHeader) {
    $$renderer.push("<!--[-->");
    $$renderer.push(`<thead class="bg-gray-50"><tr><!--[-->`);
    const each_array = ensure_array_like(Array(columns));
    for (let i = 0, $$length = each_array.length; i < $$length; i++) {
      each_array[i];
      $$renderer.push(`<th class="px-6 py-3 text-left">`);
      Skeleton($$renderer, {
        variant: "text",
        width: i === 0 ? "60%" : "50%",
        height: "0.75rem"
      });
      $$renderer.push(`<!----></th>`);
    }
    $$renderer.push(`<!--]--></tr></thead>`);
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
  const each_array_1 = ensure_array_like(Array(rows));
  for (let rowIndex = 0, $$length = each_array_1.length; rowIndex < $$length; rowIndex++) {
    each_array_1[rowIndex];
    $$renderer.push(`<tr><!--[-->`);
    const each_array_2 = ensure_array_like(Array(columns));
    for (let colIndex = 0, $$length2 = each_array_2.length; colIndex < $$length2; colIndex++) {
      each_array_2[colIndex];
      $$renderer.push(`<td class="px-6 py-4 whitespace-nowrap">`);
      if (colIndex === 0) {
        $$renderer.push("<!--[-->");
        Skeleton($$renderer, { variant: "text", width: "80%" });
      } else {
        $$renderer.push("<!--[!-->");
        if (colIndex === columns - 1) {
          $$renderer.push("<!--[-->");
          $$renderer.push(`<div class="flex justify-end gap-2">`);
          Skeleton($$renderer, { variant: "text", width: "3rem" });
          $$renderer.push(`<!----> `);
          Skeleton($$renderer, { variant: "text", width: "3rem" });
          $$renderer.push(`<!----></div>`);
        } else {
          $$renderer.push("<!--[!-->");
          Skeleton($$renderer, { variant: "text", width: `${50 + Math.random() * 30}%` });
        }
        $$renderer.push(`<!--]-->`);
      }
      $$renderer.push(`<!--]--></td>`);
    }
    $$renderer.push(`<!--]--></tr>`);
  }
  $$renderer.push(`<!--]--></tbody></table></div>`);
}
export {
  TableSkeleton as T
};
