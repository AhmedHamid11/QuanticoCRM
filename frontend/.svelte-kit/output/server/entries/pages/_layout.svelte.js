import { W as store_get, X as ensure_array_like, Y as attr_class, Z as stringify, _ as unsubscribe_stores } from "../../chunks/index.js";
import { T as Toast } from "../../chunks/Toast.js";
import "clsx";
import "@sveltejs/kit/internal";
import "../../chunks/exports.js";
import "../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../chunks/state.svelte.js";
import { g as getNavigationTabs } from "../../chunks/navigation.svelte.js";
import { p as page } from "../../chunks/stores.js";
import { e as escape_html, a as attr } from "../../chunks/attributes.js";
import { a as auth } from "../../chunks/auth.svelte.js";
function NavigationProgress($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
function _layout($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let { children } = $$props;
    let isAuthPage = store_get($$store_subs ??= {}, "$page", page).url.pathname === "/login" || store_get($$store_subs ??= {}, "$page", page).url.pathname === "/register" || store_get($$store_subs ??= {}, "$page", page).url.pathname === "/forgot-password" || store_get($$store_subs ??= {}, "$page", page).url.pathname === "/reset-password" || store_get($$store_subs ??= {}, "$page", page).url.pathname.startsWith("/accept-invite");
    let currentPath = store_get($$store_subs ??= {}, "$page", page).url.pathname;
    function isActive(href) {
      if (href === "/") return currentPath === "/";
      return currentPath.startsWith(href);
    }
    const RESERVED_ROUTES = [
      "contacts",
      "accounts",
      "admin",
      "settings",
      "tasks",
      "services",
      "accept-invite",
      "login",
      "register",
      "quotes"
    ];
    function normalizePathSegment(segment) {
      try {
        const decoded = decodeURIComponent(segment);
        return decoded.toLowerCase().replace(/[\s_]+/g, "-");
      } catch {
        return segment.toLowerCase().replace(/[\s_]+/g, "-");
      }
    }
    let currentEntitySetupLink = (() => {
      const segments = currentPath.split("/").filter(Boolean);
      if (segments.length < 2) return null;
      if (segments[0] === "admin") return null;
      const firstSegment = segments[0];
      const secondSegment = segments[1];
      const isDetailPage = secondSegment && !["new", "edit"].includes(secondSegment);
      if (!isDetailPage) return null;
      const normalizedSegment = normalizePathSegment(firstSegment);
      const matchingTab = getNavigationTabs().find((tab) => {
        if (!tab.entityName) return false;
        const normalizedHref = normalizePathSegment(tab.href.replace(/^\//, ""));
        return normalizedHref === normalizedSegment;
      });
      if (matchingTab?.entityName) {
        return `/admin/entity-manager/${matchingTab.entityName}`;
      }
      if (RESERVED_ROUTES.includes(firstSegment.toLowerCase())) {
        return null;
      }
      return null;
    })();
    let userInitials = () => {
      if (!auth.user) return "?";
      const first = auth.user.firstName?.[0] || "";
      const last = auth.user.lastName?.[0] || "";
      return (first + last).toUpperCase() || auth.user.email[0].toUpperCase();
    };
    NavigationProgress($$renderer2);
    $$renderer2.push(`<!----> `);
    if (isAuthPage) {
      $$renderer2.push("<!--[-->");
      children($$renderer2);
      $$renderer2.push(`<!---->`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (auth.isLoading) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="min-h-screen flex items-center justify-center bg-gray-50"><div class="text-center"><div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div> <p class="mt-4 text-gray-600">Loading...</p></div></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        if (auth.isAuthenticated) {
          $$renderer2.push("<!--[-->");
          $$renderer2.push(`<div class="min-h-screen bg-gray-50">`);
          if (auth.isImpersonation) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<div class="bg-amber-500 text-white px-4 py-2 text-center text-sm"><span class="font-medium">Impersonation Mode</span> - You are viewing as ${escape_html(auth.currentOrg?.orgName)} <button class="ml-4 underline hover:no-underline">Exit Impersonation</button></div>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> <nav class="bg-white shadow-sm border-b border-gray-200"><div class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8"><div class="flex justify-between h-14"><div class="flex items-center"><a href="/" class="flex items-center text-xl font-bold"><span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span></a> <div class="ml-10 flex space-x-1"><!--[-->`);
          const each_array = ensure_array_like(getNavigationTabs());
          for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
            let tab = each_array[$$index];
            $$renderer2.push(`<a${attr("href", tab.href)}${attr_class(`px-3 py-2 text-sm font-medium rounded-md transition-colors ${stringify(isActive(tab.href) ? "bg-blue-50 text-blue-700" : "text-gray-600 hover:text-gray-900 hover:bg-gray-50")}`)}>${escape_html(tab.label)}</a>`);
          }
          $$renderer2.push(`<!--]--></div></div> <div class="flex items-center space-x-2">`);
          if (auth.memberships.length > 1) {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<div class="relative org-switcher-container"><button class="flex items-center px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 rounded-md transition-colors"><span class="font-medium whitespace-nowrap truncate max-w-[200px]">${escape_html(auth.currentOrg?.orgName)}</span> <svg class="ml-1 w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg></button> `);
            {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]--></div>`);
          } else {
            $$renderer2.push("<!--[!-->");
            if (auth.currentOrg) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<span class="text-sm text-gray-600 whitespace-nowrap truncate max-w-[200px]">${escape_html(auth.currentOrg.orgName)}</span>`);
            } else {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]-->`);
          }
          $$renderer2.push(`<!--]--> `);
          if (auth.canAccessSetup) {
            $$renderer2.push("<!--[-->");
            if (currentEntitySetupLink) {
              $$renderer2.push("<!--[-->");
              $$renderer2.push(`<a${attr("href", currentEntitySetupLink)} class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-md transition-colors" title="Edit Object"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path></svg></a>`);
            } else {
              $$renderer2.push("<!--[!-->");
            }
            $$renderer2.push(`<!--]--> <a href="/admin" class="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-md transition-colors" title="Setup"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path></svg></a>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--> <div class="relative user-menu-container"><button class="flex items-center space-x-2 p-1 rounded-full hover:bg-gray-100 transition-colors"><div class="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white text-sm font-medium">${escape_html(userInitials())}</div></button> `);
          {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></div></div></div></div></nav> <main class="w-full max-w-[75%] mx-auto px-4 sm:px-6 lg:px-8 py-6">`);
          children($$renderer2);
          $$renderer2.push(`<!----></main></div>`);
        } else {
          $$renderer2.push("<!--[!-->");
          children($$renderer2);
          $$renderer2.push(`<!---->`);
        }
        $$renderer2.push(`<!--]-->`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--> `);
    Toast($$renderer2);
    $$renderer2.push(`<!---->`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _layout as default
};
