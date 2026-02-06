import "clsx";
import "./auth.svelte.js";
let tabs = [];
function getNavigationTabs() {
  return tabs;
}
function getEntityNameFromPath(path) {
  const normalizedPath = "/" + path.replace(/^\//, "");
  const tab = tabs.find((t) => t.href === normalizedPath);
  return tab?.entityName || null;
}
export {
  getEntityNameFromPath as a,
  getNavigationTabs as g
};
