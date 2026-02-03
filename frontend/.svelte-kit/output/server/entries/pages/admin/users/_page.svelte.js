import { X as ensure_array_like, Y as attr_class, Z as stringify } from "../../../../chunks/index.js";
import "../../../../chunks/auth.svelte.js";
import { e as escape_html } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let pendingInvitations = [];
    const roleLabels = {
      owner: { label: "Owner", color: "bg-purple-100 text-purple-800" },
      admin: { label: "Admin", color: "bg-blue-100 text-blue-800" },
      user: { label: "User", color: "bg-gray-100 text-gray-800" }
    };
    function formatDate(dateStr) {
      if (!dateStr) return "Never";
      return new Date(dateStr).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
    }
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div><h1 class="text-2xl font-bold text-gray-900">User Management</h1> <p class="mt-1 text-sm text-gray-500">Manage users and their roles in your organization</p></div> <div class="flex items-center space-x-3"><button class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg> Invite User</button> <a href="/admin" class="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"><svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"></path></svg> Back to Setup</a></div></div> `);
    if (pendingInvitations.length > 0) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="bg-white shadow rounded-lg overflow-hidden"><div class="px-6 py-4 border-b border-gray-200 bg-amber-50"><h2 class="text-lg font-medium text-amber-800">Pending Invitations (${escape_html(pendingInvitations.length)})</h2> <p class="text-sm text-amber-600">These users have been invited but haven't accepted yet. Share the invite link with them.</p></div> <table class="min-w-full divide-y divide-gray-200"><thead class="bg-gray-50"><tr><th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Email</th><th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Role</th><th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Invited</th><th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Expires</th><th scope="col" class="relative px-6 py-3"><span class="sr-only">Actions</span></th></tr></thead><tbody class="bg-white divide-y divide-gray-200"><!--[-->`);
      const each_array = ensure_array_like(pendingInvitations);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let invitation = each_array[$$index];
        $$renderer2.push(`<tr class="hover:bg-gray-50"><td class="px-6 py-4 whitespace-nowrap"><div class="text-sm font-medium text-gray-900">${escape_html(invitation.email)}</div></td><td class="px-6 py-4 whitespace-nowrap"><span${attr_class(`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${stringify(roleLabels[invitation.role]?.color || "bg-gray-100 text-gray-800")}`)}>${escape_html(roleLabels[invitation.role]?.label || invitation.role)}</span></td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(formatDate(invitation.createdAt))}</td><td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${escape_html(formatDate(invitation.expiresAt))}</td><td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2"><button class="text-blue-600 hover:text-blue-900" title="Copy invite link">Copy Link</button> <button class="text-red-600 hover:text-red-900">Cancel</button></td></tr>`);
      }
      $$renderer2.push(`<!--]--></tbody></table></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="bg-white shadow rounded-lg overflow-hidden">`);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="flex items-center justify-center py-12"><div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div></div>`);
    }
    $$renderer2.push(`<!--]--></div> <div class="bg-blue-50 rounded-lg p-4"><h3 class="text-sm font-medium text-blue-800 mb-2">Role Descriptions</h3> <div class="space-y-1 text-sm text-blue-700"><p><span class="font-medium">Owner:</span> Full organization control including delete org, transfer ownership, and all admin capabilities</p> <p><span class="font-medium">Admin:</span> Access to Setup (entity manager, navigation, tripwires, etc.) and user management</p> <p><span class="font-medium">User:</span> Access to CRM objects only (contacts, accounts, tasks) - cannot access Setup</p></div></div></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
