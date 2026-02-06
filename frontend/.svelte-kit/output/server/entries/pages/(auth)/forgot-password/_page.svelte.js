import { $ as head } from "../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { a as attr } from "../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let email = "";
    let isSubmitting = false;
    head("1xufxwe", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Forgot Password - Quantico CRM</title>`);
      });
    });
    $$renderer2.push(`<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8"><div class="max-w-md w-full space-y-8"><div><h1 class="text-center text-3xl font-bold"><span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1> <h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Reset your password</h2> <p class="mt-2 text-center text-sm text-gray-600">Enter your email address and we'll send you a link to reset your password.</p></div> <form class="mt-8 space-y-6">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="space-y-4"><div><label for="email" class="block text-sm font-medium text-gray-700">Email address</label> <input id="email" name="email" type="email" autocomplete="email" required${attr("value", email)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="you@company.com"/></div></div> <div><button type="submit"${attr("disabled", isSubmitting, true)} class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-600/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed">`);
      {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`Send reset link`);
      }
      $$renderer2.push(`<!--]--></button></div>`);
    }
    $$renderer2.push(`<!--]--> <div class="text-center"><a href="/login" class="font-medium text-blue-600 hover:text-blue-600">Back to sign in</a></div></form></div></div>`);
  });
}
export {
  _page as default
};
