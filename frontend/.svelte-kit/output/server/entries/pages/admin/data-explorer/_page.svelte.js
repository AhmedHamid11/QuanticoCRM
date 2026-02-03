import { e as escape_html, a as attr } from "../../../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let sqlQuery = "SELECT * FROM contacts LIMIT 10";
    let loading = false;
    $$renderer2.push(`<div class="space-y-6"><nav class="text-sm text-gray-500"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <span class="text-gray-900">Data Explorer</span></nav> <div class="flex items-center justify-between"><div><h1 class="text-2xl font-bold text-gray-900">Data Explorer</h1> <p class="mt-1 text-sm text-gray-500">Execute SQL queries and explore your data</p></div></div> <div class="bg-white shadow rounded-lg p-6"><label for="sql-query" class="block text-sm font-medium text-gray-700 mb-2">SQL Query</label> <textarea id="sql-query" rows="6" class="w-full font-mono text-sm border border-gray-300 rounded-lg p-3 focus:ring-2 focus:ring-teal-500 focus:border-teal-500" placeholder="SELECT * FROM table_name LIMIT 10">`);
    const $$body = escape_html(sqlQuery);
    if ($$body) {
      $$renderer2.push(`${$$body}`);
    }
    $$renderer2.push(`</textarea> <p class="mt-1 text-xs text-gray-400">Press Ctrl+Enter to execute</p> <div class="mt-4 flex items-center justify-between"><div class="flex gap-3"><button${attr("disabled", loading, true)} class="px-4 py-2 bg-teal-600 text-white rounded-lg hover:bg-teal-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`Execute`);
    }
    $$renderer2.push(`<!--]--></button> <button class="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50">Clear</button> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[!-->");
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
