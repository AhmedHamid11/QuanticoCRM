import { W as store_get, _ as unsubscribe_stores } from "./index.js";
import "@sveltejs/kit/internal";
import "./exports.js";
import "./utils.js";
import { a as attr } from "./attributes.js";
import "@sveltejs/kit/internal/server";
import "./state.svelte.js";
import { p as page } from "./stores.js";
import "./auth.svelte.js";
function LoginForm($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let email = "";
    let password = "";
    let isSubmitting = false;
    let sessionExpired = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("session_expired") === "true";
    $$renderer2.push(`<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8"><div class="max-w-md w-full space-y-8"><div><h1 class="text-center text-3xl font-bold"><span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1> <h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Sign in to your account</h2> <p class="mt-2 text-center text-sm text-gray-600">Or <a href="/register" class="font-medium text-blue-600 hover:text-blue-500">create a new account</a></p></div> <form class="mt-8 space-y-6">`);
    if (sessionExpired) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="rounded-md bg-amber-50 p-4 border border-amber-200"><div class="flex"><div class="flex-shrink-0"><svg class="h-5 w-5 text-amber-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path></svg></div> <div class="ml-3"><p class="text-sm font-medium text-amber-800">Your session has expired. Please sign in again.</p></div></div></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="space-y-4"><div><label for="email" class="block text-sm font-medium text-gray-700">Email address</label> <input id="email" name="email" type="email" autocomplete="email" required${attr("value", email)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="you@company.com"/></div> <div><div class="flex justify-between"><label for="password" class="block text-sm font-medium text-gray-700">Password</label> <a href="/forgot-password" class="text-sm font-medium text-blue-600 hover:text-blue-600">Forgot password?</a></div> <input id="password" name="password" type="password" autocomplete="current-password" required${attr("value", password)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Your password"/></div></div> <div><button type="submit"${attr("disabled", isSubmitting, true)} class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-600/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`Sign in`);
    }
    $$renderer2.push(`<!--]--></button></div></form></div></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  LoginForm as L
};
