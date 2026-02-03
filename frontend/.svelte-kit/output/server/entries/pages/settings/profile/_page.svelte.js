import { Y as attr_class, Z as stringify, X as ensure_array_like } from "../../../../chunks/index.js";
import { a as auth } from "../../../../chunks/auth.svelte.js";
import { e as escape_html, a as attr } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
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
    $$renderer2.push(`<!--]--> <div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200"><h2 class="text-lg font-medium text-gray-900">Change Password</h2></div> <div class="px-6 py-4"><form class="space-y-4 max-w-md">`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div><label for="currentPassword" class="block text-sm font-medium text-gray-700">Current Password</label> <input type="password" id="currentPassword"${attr("value", currentPassword)} required class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="newPassword" class="block text-sm font-medium text-gray-700">New Password</label> <input type="password" id="newPassword"${attr("value", newPassword)} required minlength="8" class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <div><label for="confirmPassword" class="block text-sm font-medium text-gray-700">Confirm New Password</label> <input type="password" id="confirmPassword"${attr("value", confirmPassword)} required minlength="8" class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:ring-blue-500 focus:border-blue-500"/></div> <button type="submit"${attr("disabled", changingPassword, true)} class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors">${escape_html("Change Password")}</button></form></div></div> `);
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
