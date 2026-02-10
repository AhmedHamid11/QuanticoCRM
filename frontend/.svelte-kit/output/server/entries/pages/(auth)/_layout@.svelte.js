import "clsx";
import { T as Toast } from "../../../chunks/Toast.js";
function _layout_($$renderer, $$props) {
  let { children } = $$props;
  children($$renderer);
  $$renderer.push(`<!----> `);
  Toast($$renderer);
  $$renderer.push(`<!---->`);
}
export {
  _layout_ as default
};
