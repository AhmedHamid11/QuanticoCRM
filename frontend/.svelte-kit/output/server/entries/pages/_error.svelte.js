import { W as store_get, _ as unsubscribe_stores } from "../../chunks/index.js";
import { p as page } from "../../chunks/stores.js";
import { B as Button } from "../../chunks/Button.js";
import { e as escape_html } from "../../chunks/attributes.js";
function _error($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let status = store_get($$store_subs ??= {}, "$page", page).status;
    let message = store_get($$store_subs ??= {}, "$page", page).error?.message || "Something went wrong";
    const errorInfo = {
      404: {
        title: "Page Not Found",
        description: "The page you are looking for does not exist or has been moved.",
        icon: "search"
      },
      403: {
        title: "Access Denied",
        description: "You do not have permission to access this resource.",
        icon: "lock"
      },
      500: {
        title: "Server Error",
        description: "An unexpected error occurred. Please try again later.",
        icon: "alert"
      }
    };
    let info = errorInfo[status] || errorInfo[500];
    function handleRetry() {
      window.location.reload();
    }
    function handleGoHome() {
      window.location.href = "/";
    }
    $$renderer2.push(`<div class="min-h-[60vh] flex items-center justify-center px-4"><div class="max-w-md w-full text-center"><div class="mx-auto w-20 h-20 rounded-full bg-gray-100 flex items-center justify-center mb-6">`);
    if (info.icon === "search") {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<svg class="w-10 h-10 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path></svg>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (info.icon === "lock") {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<svg class="w-10 h-10 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path></svg>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<svg class="w-10 h-10 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div> <p class="text-6xl font-bold text-gray-300 mb-4">${escape_html(status)}</p> <h1 class="text-2xl font-bold text-gray-900 mb-2">${escape_html(info.title)}</h1> <p class="text-gray-500 mb-8">${escape_html(info.description)}</p> `);
    if (message !== info.description) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p class="text-sm text-gray-400 mb-6 px-4 py-2 bg-gray-50 rounded-md font-mono">${escape_html(message)}</p>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="flex justify-center gap-3">`);
    if (status === 500) {
      $$renderer2.push("<!--[-->");
      Button($$renderer2, {
        variant: "primary",
        onclick: handleRetry,
        children: ($$renderer3) => {
          $$renderer3.push(`<!---->Try Again`);
        }
      });
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    Button($$renderer2, {
      variant: status === 500 ? "secondary" : "primary",
      onclick: handleGoHome,
      children: ($$renderer3) => {
        $$renderer3.push(`<!---->Go Home`);
      }
    });
    $$renderer2.push(`<!----></div></div></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _error as default
};
