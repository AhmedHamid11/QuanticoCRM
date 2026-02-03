import { W as store_get, _ as unsubscribe_stores } from "../../../chunks/index.js";
import { p as page } from "../../../chunks/stores.js";
import { B as Button } from "../../../chunks/Button.js";
import { e as escape_html } from "../../../chunks/attributes.js";
function _error($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let status = store_get($$store_subs ??= {}, "$page", page).status;
    let message = store_get($$store_subs ??= {}, "$page", page).error?.message || "Something went wrong";
    const errorInfo = {
      404: {
        title: "Page Not Found",
        description: "The admin page you are looking for does not exist."
      },
      403: {
        title: "Access Denied",
        description: "You do not have permission to access this admin feature."
      },
      500: {
        title: "Server Error",
        description: "An unexpected error occurred in the admin panel."
      }
    };
    let info = errorInfo[status] || errorInfo[500];
    function handleRetry() {
      window.location.reload();
    }
    function handleGoBack() {
      window.history.back();
    }
    function handleGoToAdmin() {
      window.location.href = "/admin";
    }
    $$renderer2.push(`<div class="max-w-lg mx-auto py-12 text-center"><div class="mx-auto w-16 h-16 rounded-full bg-red-100 flex items-center justify-center mb-4"><svg class="w-8 h-8 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg></div> <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 mb-4">Error ${escape_html(status)}</span> <h1 class="text-xl font-bold text-gray-900 mb-2">${escape_html(info.title)}</h1> <p class="text-gray-500 mb-6">${escape_html(info.description)}</p> `);
    if (message !== info.description) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="mb-6 px-4 py-3 bg-gray-50 rounded-md"><p class="text-sm text-gray-600 font-mono">${escape_html(message)}</p></div>`);
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
      variant: "secondary",
      onclick: handleGoBack,
      children: ($$renderer3) => {
        $$renderer3.push(`<!---->Go Back`);
      }
    });
    $$renderer2.push(`<!----> `);
    Button($$renderer2, {
      variant: "ghost",
      onclick: handleGoToAdmin,
      children: ($$renderer3) => {
        $$renderer3.push(`<!---->Admin Home`);
      }
    });
    $$renderer2.push(`<!----></div></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _error as default
};
