import { Z as stringify } from "./index.js";
import { e as escape_html, a as attr } from "./attributes.js";
function LookupField($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      entity,
      value,
      valueName,
      label,
      required = false,
      disabled = false,
      onchange
    } = $$props;
    let searchTerm = "";
    $$renderer2.push(`<div class="relative"><label class="block text-sm font-medium text-gray-700 mb-1">${escape_html(label)} `);
    if (required) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<span class="text-red-500">*</span>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></label> <div class="relative">`);
    if (value) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="flex items-center gap-2 w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50"><a${attr("href", `/${stringify(entity.toLowerCase())}s/${stringify(value)}`)} class="text-blue-600 hover:text-blue-800 hover:underline flex-1 truncate">${escape_html(valueName)}</a> `);
      if (!disabled) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<button type="button" class="text-gray-400 hover:text-gray-600 flex-shrink-0" aria-label="Clear selection"><svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg></button>`);
      } else {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<input type="text"${attr("value", searchTerm)}${attr("placeholder", `Search ${stringify(entity)}...`)} class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"${attr("disabled", disabled, true)}${attr("required", required && !value, true)}/> `);
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div></div>`);
  });
}
export {
  LookupField as L
};
