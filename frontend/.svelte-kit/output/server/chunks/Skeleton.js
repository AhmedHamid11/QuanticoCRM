import { Y as attr_class, a1 as attr_style, Z as stringify } from "./index.js";
function Skeleton($$renderer, $$props) {
  let {
    variant = "text",
    width,
    height,
    rounded = "md",
    class: className = ""
  } = $$props;
  const variantDefaults = {
    text: { width: "100%", height: "1rem", rounded: "rounded" },
    heading: { width: "60%", height: "1.5rem", rounded: "rounded" },
    avatar: { width: "2.5rem", height: "2.5rem", rounded: "rounded-full" },
    input: { width: "100%", height: "2.5rem", rounded: "rounded-md" },
    card: { width: "100%", height: "6rem", rounded: "rounded-lg" },
    button: { width: "5rem", height: "2.25rem", rounded: "rounded-md" }
  };
  const roundedClasses = {
    none: "rounded-none",
    sm: "rounded-sm",
    md: "rounded-md",
    lg: "rounded-lg",
    full: "rounded-full"
  };
  let defaults = variantDefaults[variant] || variantDefaults.text;
  let appliedRounded = rounded === "md" && variant === "avatar" ? "rounded-full" : roundedClasses[rounded];
  $$renderer.push(`<div${attr_class(`animate-pulse bg-gray-200 ${stringify(appliedRounded)} ${stringify(className)}`)}${attr_style(`width: ${stringify(width || defaults.width)}; height: ${stringify(height || defaults.height)};`)}></div>`);
}
export {
  Skeleton as S
};
