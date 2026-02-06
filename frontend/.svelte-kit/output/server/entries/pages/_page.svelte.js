import { $ as head } from "../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../chunks/exports.js";
import "../../chunks/utils.js";
import { a as attr, e as escape_html } from "../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../chunks/state.svelte.js";
import { g as getNavigationTabs } from "../../chunks/navigation.svelte.js";
import { a as auth } from "../../chunks/auth.svelte.js";
import { L as LoginForm } from "../../chunks/LoginForm.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let firstTab = getNavigationTabs()[0];
    head("1uha8ag", $$renderer2, ($$renderer3) => {
      if (!auth.isAuthenticated && !auth.isLoading) {
        $$renderer3.push("<!--[-->");
        $$renderer3.title(($$renderer4) => {
          $$renderer4.push(`<title>Login - Quantico CRM</title>`);
        });
      } else {
        $$renderer3.push("<!--[!-->");
      }
      $$renderer3.push(`<!--]-->`);
    });
    if (auth.isLoading) {
      $$renderer2.push("<!--[-->");
    } else {
      $$renderer2.push("<!--[!-->");
      if (!auth.isAuthenticated) {
        $$renderer2.push("<!--[-->");
        LoginForm($$renderer2);
      } else {
        $$renderer2.push("<!--[!-->");
        {
          $$renderer2.push("<!--[!-->");
          $$renderer2.push(`<div class="text-center py-12"><h1 class="text-4xl font-bold text-gray-900 mb-4">Welcome to <span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1> <p class="text-lg text-gray-600 mb-8">A CRM built with discipline and precision</p> <a${attr("href", firstTab?.href || "/contacts")} class="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90">View ${escape_html(firstTab?.label || "Contacts")}</a></div>`);
        }
        $$renderer2.push(`<!--]-->`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
