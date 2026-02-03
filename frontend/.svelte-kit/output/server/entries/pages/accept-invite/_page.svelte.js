import { W as store_get, $ as head, _ as unsubscribe_stores } from "../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import { a as attr } from "../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../chunks/state.svelte.js";
import { p as page } from "../../../chunks/stores.js";
import "../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let token = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("token") || "";
    let password = "";
    let confirmPassword = "";
    let firstName = "";
    let lastName = "";
    let isSubmitting = false;
    head("p8b09p", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Accept Invitation - Quantico CRM</title>`);
      });
    });
    $$renderer2.push(`<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8"><div class="max-w-md w-full space-y-8"><div><h1 class="text-center text-3xl font-bold"><span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1> <h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Accept Invitation</h2> <p class="mt-2 text-center text-sm text-gray-600">Set up your account to join the organization</p></div> `);
    if (!token) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="rounded-md bg-red-50 p-4"><div class="flex"><div class="flex-shrink-0"><svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path></svg></div> <div class="ml-3"><p class="text-sm font-medium text-red-800">Invalid invitation link. Please request a new invitation from your administrator.</p></div></div></div> <div class="text-center"><a href="/login" class="text-blue-600 hover:text-blue-500 font-medium">Go to login</a></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<form class="mt-8 space-y-6">`);
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--> <div class="space-y-4"><div class="grid grid-cols-2 gap-4"><div><label for="firstName" class="block text-sm font-medium text-gray-700">First Name</label> <input id="firstName" name="firstName" type="text"${attr("value", firstName)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="John"/></div> <div><label for="lastName" class="block text-sm font-medium text-gray-700">Last Name</label> <input id="lastName" name="lastName" type="text"${attr("value", lastName)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Doe"/></div></div> <div><label for="password" class="block text-sm font-medium text-gray-700">Password</label> <input id="password" name="password" type="password" required minlength="8"${attr("value", password)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="At least 8 characters"/></div> <div><label for="confirmPassword" class="block text-sm font-medium text-gray-700">Confirm Password</label> <input id="confirmPassword" name="confirmPassword" type="password" required minlength="8"${attr("value", confirmPassword)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Confirm your password"/></div></div> <div><button type="submit"${attr("disabled", isSubmitting, true)} class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed">`);
      {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`Accept Invitation`);
      }
      $$renderer2.push(`<!--]--></button></div> <p class="text-center text-sm text-gray-600">Already have an account? <a href="/login" class="font-medium text-blue-600 hover:text-blue-500">Sign in</a></p></form>`);
    }
    $$renderer2.push(`<!--]--></div></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
