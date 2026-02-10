import { $ as head } from "../../../../chunks/index.js";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import "../../../../chunks/auth.svelte.js";
import { L as LoginForm } from "../../../../chunks/LoginForm.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    head("8k30lk", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Login - Quantico CRM</title>`);
      });
    });
    LoginForm($$renderer2);
  });
}
export {
  _page as default
};
