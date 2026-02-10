import { Y as attr_class, Z as stringify } from "./index.js";
import { a as attr } from "./attributes.js";
function Spinner($$renderer, $$props) {
  let { size = "md", color = "blue", class: className = "" } = $$props;
  const sizeClasses = {
    sm: "h-4 w-4 border-2",
    md: "h-6 w-6 border-2",
    lg: "h-10 w-10 border-3"
  };
  const colorClasses = {
    blue: "border-blue-600 border-t-transparent",
    white: "border-white border-t-transparent",
    gray: "border-gray-400 border-t-transparent"
  };
  $$renderer.push(`<div${attr_class(`animate-spin rounded-full ${stringify(sizeClasses[size])} ${stringify(colorClasses[color])} ${stringify(className)}`)} role="status" aria-label="Loading"></div>`);
}
function Button($$renderer, $$props) {
  let {
    type = "button",
    variant = "primary",
    size = "md",
    loading = false,
    disabled = false,
    onclick,
    class: className = "",
    children
  } = $$props;
  const baseClasses = "inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed";
  const variantClasses = {
    primary: "bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500",
    secondary: "bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 focus:ring-blue-500",
    danger: "bg-red-600 text-white hover:bg-red-700 focus:ring-red-500",
    ghost: "text-gray-700 hover:bg-gray-100 focus:ring-gray-500"
  };
  const sizeClasses = {
    sm: "px-3 py-1.5 text-sm",
    md: "px-4 py-2 text-sm",
    lg: "px-6 py-3 text-base"
  };
  let spinnerColor = variant === "primary" || variant === "danger" ? "white" : "gray";
  $$renderer.push(`<button${attr("type", type)}${attr("disabled", disabled || loading, true)}${attr_class(`${stringify(baseClasses)} ${stringify(variantClasses[variant])} ${stringify(sizeClasses[size])} ${stringify(className)}`)}>`);
  if (loading) {
    $$renderer.push("<!--[-->");
    Spinner($$renderer, { size: "sm", color: spinnerColor, class: "mr-2" });
  } else {
    $$renderer.push("<!--[!-->");
  }
  $$renderer.push(`<!--]--> `);
  children($$renderer);
  $$renderer.push(`<!----></button>`);
}
export {
  Button as B
};
