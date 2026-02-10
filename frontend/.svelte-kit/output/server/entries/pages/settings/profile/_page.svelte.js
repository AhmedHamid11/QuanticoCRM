import { Y as attr_class, Z as stringify, X as ensure_array_like } from "../../../../chunks/index.js";
import { a as auth } from "../../../../chunks/auth.svelte.js";
import { e as escape_html, a as attr } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let downloadingExtension = false;
    let currentPassword = "";
    let newPassword = "";
    let confirmPassword = "";
    let changingPassword = false;
    $$renderer2.push(`<div class="space-y-6"><div><h1 class="text-2xl font-bold text-gray-900">Profile Settings</h1> <p class="mt-1 text-sm text-gray-500">Manage your account settings and preferences</p></div> <div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">Profile Information</h2></div> <div class="px-6 py-4"><dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4"><div><dt class="text-sm font-medium text-gray-500">First Name</dt> <dd class="mt-1 text-sm text-gray-900">${escape_html(auth.user?.firstName || "-")}</dd></div> <div><dt class="text-sm font-medium text-gray-500">Last Name</dt> <dd class="mt-1 text-sm text-gray-900">${escape_html(auth.user?.lastName || "-")}</dd></div> <div><dt class="text-sm font-medium text-gray-500">Email</dt> <dd class="mt-1 text-sm text-gray-900">${escape_html(auth.user?.email || "-")}</dd></div> <div><dt class="text-sm font-medium text-gray-500">Email Verified</dt> <dd class="mt-1 text-sm">`);
    if (auth.user?.emailVerified) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">Verified</span>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">Not Verified</span>`);
    }
    $$renderer2.push(`<!--]--></dd></div></dl></div></div> <div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">Current Organization</h2></div> <div class="px-6 py-4"><dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4"><div><dt class="text-sm font-medium text-gray-500">Organization</dt> <dd class="mt-1 text-sm text-gray-900">${escape_html(auth.currentOrg?.orgName || "-")}</dd></div> <div><dt class="text-sm font-medium text-gray-500">Role</dt> <dd class="mt-1 text-sm"><span${attr_class(`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${stringify(auth.currentOrg?.role === "owner" ? "bg-purple-100 text-purple-800" : auth.currentOrg?.role === "admin" ? "bg-blue-100 text-blue-800" : "bg-gray-100 text-gray-800")}`)}>${escape_html(auth.currentOrg?.role || "-")}</span></dd></div></dl></div></div> `);
    if (auth.memberships.length > 1) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">All Organizations</h2></div> <div class="px-6 py-4"><ul class="divide-y divide-gray-200"><!--[-->`);
      const each_array = ensure_array_like(auth.memberships);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let membership = each_array[$$index];
        $$renderer2.push(`<li class="py-3 flex justify-between items-center"><div><p class="text-sm font-medium text-gray-900">${escape_html(membership.orgName)}</p> <p class="text-sm text-gray-500 capitalize">${escape_html(membership.role)}</p></div> `);
        if (membership.isDefault) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">Default</span>`);
        } else {
          $$renderer2.push("<!--[!-->");
        }
        $$renderer2.push(`<!--]--></li>`);
      }
      $$renderer2.push(`<!--]--></ul></div></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">Integrations</h2> <p class="mt-1 text-sm text-gray-500">Connect QuanticoCRM with other tools</p></div> <div class="px-6 py-4"><div class="flex items-start gap-4"><div class="flex-shrink-0"><svg class="w-10 h-10 text-red-500" viewBox="0 0 24 24" fill="currentColor"><path d="M24 5.457v13.909c0 .904-.732 1.636-1.636 1.636h-3.819V11.73L12 16.64l-6.545-4.91v9.273H1.636A1.636 1.636 0 0 1 0 19.366V5.457c0-2.023 2.309-3.178 3.927-1.964L5.455 4.64 12 9.548l6.545-4.91 1.528-1.145C21.69 2.28 24 3.434 24 5.457z"></path></svg></div> <div class="flex-1"><div class="flex items-center gap-2"><h3 class="text-base font-medium text-gray-900">Quantico CRM for Gmail</h3> <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">v1.0.1</span></div> <p class="mt-1 text-sm text-gray-600">Access your Quantico CRM data directly in Gmail. Log emails, view contacts, and manage deals without leaving your inbox.</p> <ul class="mt-3 space-y-1"><li class="flex items-center gap-2 text-sm text-gray-600"><svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg> Log emails to CRM</li> <li class="flex items-center gap-2 text-sm text-gray-600"><svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg> View contact info in Gmail</li> <li class="flex items-center gap-2 text-sm text-gray-600"><svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg> Create tasks from emails</li></ul> <div class="mt-4 flex items-center gap-3"><a href="https://chrome.google.com/webstore" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 transition-colors"><svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path></svg> Chrome Web Store</a> <button${attr("disabled", downloadingExtension, true)} class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path></svg> Download ZIP`);
    }
    $$renderer2.push(`<!--]--></button></div> <p class="mt-3 text-xs text-gray-500"><strong>Manual install:</strong> Download the ZIP, extract it, then go to <code class="bg-gray-100 px-1 rounded">chrome://extensions</code>, enable "Developer mode", and click "Load unpacked" to select the extracted folder.</p></div></div></div></div> <div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">Change Password</h2></div> <div class="px-6 py-4"><form class="space-y-4 max-w-md">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div><label for="currentPassword" class="block text-sm font-medium text-gray-700">Current Password</label> <input type="password" id="currentPassword"${attr("value", currentPassword)} required class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="newPassword" class="block text-sm font-medium text-gray-700">New Password</label> <input type="password" id="newPassword"${attr("value", newPassword)} required minlength="8" class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="confirmPassword" class="block text-sm font-medium text-gray-700">Confirm New Password</label> <input type="password" id="confirmPassword"${attr("value", confirmPassword)} required minlength="8" class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <button type="submit"${attr("disabled", changingPassword, true)} class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors">${escape_html("Change Password")}</button></form></div></div> `);
    if (auth.isPlatformAdmin) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-purple-50 border border-purple-200 rounded-lg p-4"><div class="flex items-center gap-2"><svg class="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path></svg> <span class="text-sm font-medium text-purple-800">Platform Administrator</span></div> <p class="mt-1 text-sm text-purple-600">You have platform-wide administrative privileges.</p></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
