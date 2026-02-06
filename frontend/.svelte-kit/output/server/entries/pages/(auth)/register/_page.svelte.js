import { $ as head } from "../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import { a as attr } from "../../../../chunks/attributes.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import "../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let email = "";
    let password = "";
    let confirmPassword = "";
    let firstName = "";
    let lastName = "";
    let orgName = "";
    let isSubmitting = false;
    head("ydeots", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Register - Quantico CRM</title>`);
      });
    });
    $$renderer2.push(`<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8"><div class="max-w-md w-full space-y-8"><div><h1 class="text-center text-3xl font-bold"><span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></h1> <h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Create your account</h2> <p class="mt-2 text-center text-sm text-gray-600">Already have an account? <a href="/login" class="font-medium text-blue-600 hover:text-blue-600">Sign in</a></p></div> <form class="mt-8 space-y-6">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="space-y-4"><div><label for="orgName" class="block text-sm font-medium text-gray-700">Organization Name</label> <input id="orgName" name="orgName" type="text" required${attr("value", orgName)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Acme Corp"/></div> <div class="grid grid-cols-2 gap-4"><div><label for="firstName" class="block text-sm font-medium text-gray-700">First Name</label> <input id="firstName" name="firstName" type="text"${attr("value", firstName)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="John"/></div> <div><label for="lastName" class="block text-sm font-medium text-gray-700">Last Name</label> <input id="lastName" name="lastName" type="text"${attr("value", lastName)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Doe"/></div></div> <div><label for="email" class="block text-sm font-medium text-gray-700">Email address</label> <input id="email" name="email" type="email" autocomplete="email" required${attr("value", email)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="you@company.com"/></div> <div><label for="password" class="block text-sm font-medium text-gray-700">Password</label> <input id="password" name="password" type="password" autocomplete="new-password" required${attr("value", password)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="At least 8 characters"/></div> <div><label for="confirmPassword" class="block text-sm font-medium text-gray-700">Confirm Password</label> <input id="confirmPassword" name="confirmPassword" type="password" autocomplete="new-password" required${attr("value", confirmPassword)} class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm" placeholder="Confirm your password"/></div></div> <div><button type="submit"${attr("disabled", isSubmitting, true)} class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-600/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`Create account`);
    }
    $$renderer2.push(`<!--]--></button></div> <p class="text-xs text-center text-gray-500">By creating an account, you agree to our terms of service and privacy policy.</p></form></div></div>`);
  });
}
export {
  _page as default
};
