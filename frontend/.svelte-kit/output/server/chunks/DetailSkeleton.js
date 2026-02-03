import { Y as attr_class, X as ensure_array_like, Z as stringify } from "./index.js";
import { S as Skeleton } from "./Skeleton.js";
function DetailSkeleton($$renderer, $$props) {
  let {
    sections = 2,
    fieldsPerSection = 4,
    showAvatar = true,
    class: className = ""
  } = $$props;
  $$renderer.push(`<div${attr_class(`space-y-6 ${stringify(className)}`)}><div class="flex items-center justify-between"><div class="flex items-center gap-4">`);
  if (showAvatar) {
    $$renderer.push("<!--[-->");
    Skeleton($$renderer, { variant: "avatar", width: "4rem", height: "4rem" });
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--> <div class="space-y-2">`);
  Skeleton($$renderer, { variant: "heading", width: "12rem" });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "text", width: "8rem", height: "0.875rem" });
  $$renderer.push(`<!----></div></div> <div class="flex gap-2">`);
  Skeleton($$renderer, { variant: "button", width: "4rem" });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "button", width: "4rem" });
  $$renderer.push(`<!----></div></div> <div class="border-b border-gray-200 pb-2"><div class="flex gap-6">`);
  Skeleton($$renderer, { variant: "text", width: "4rem", height: "1rem" });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "text", width: "5rem", height: "1rem" });
  $$renderer.push(`<!----></div></div> <!--[-->`);
  const each_array = ensure_array_like(Array(sections));
  for (let sectionIndex = 0, $$length = each_array.length; sectionIndex < $$length; sectionIndex++) {
    each_array[sectionIndex];
    $$renderer.push(`<div class="bg-white shadow rounded-lg p-6">`);
    Skeleton($$renderer, { variant: "heading", width: "30%", class: "mb-4" });
    $$renderer.push(`<!----> <div class="grid grid-cols-2 gap-4"><!--[-->`);
    const each_array_1 = ensure_array_like(Array(fieldsPerSection));
    for (let fieldIndex = 0, $$length2 = each_array_1.length; fieldIndex < $$length2; fieldIndex++) {
      each_array_1[fieldIndex];
      $$renderer.push(`<div>`);
      Skeleton($$renderer, {
        variant: "text",
        width: "40%",
        height: "0.75rem",
        class: "mb-1"
      });
      $$renderer.push(`<!----> `);
      Skeleton($$renderer, { variant: "text", width: "70%", height: "1rem" });
      $$renderer.push(`<!----></div>`);
    }
    $$renderer.push(`<!--]--></div></div>`);
  }
  $$renderer.push(`<!--]--> <div class="bg-white shadow rounded-lg p-6">`);
  Skeleton($$renderer, { variant: "heading", width: "40%", class: "mb-4" });
  $$renderer.push(`<!----> <div class="grid grid-cols-2 gap-4"><div>`);
  Skeleton($$renderer, {
    variant: "text",
    width: "30%",
    height: "0.75rem",
    class: "mb-1"
  });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "text", width: "60%" });
  $$renderer.push(`<!----></div> <div>`);
  Skeleton($$renderer, {
    variant: "text",
    width: "30%",
    height: "0.75rem",
    class: "mb-1"
  });
  $$renderer.push(`<!----> `);
  Skeleton($$renderer, { variant: "text", width: "60%" });
  $$renderer.push(`<!----></div></div></div></div>`);
}
export {
  DetailSkeleton as D
};
