const RESERVED_ROUTES = ["contacts", "accounts", "admin", "settings", "tasks", "services", "accept-invite", "login", "register"];
function match(param) {
  return !RESERVED_ROUTES.includes(param.toLowerCase());
}
export {
  match
};
