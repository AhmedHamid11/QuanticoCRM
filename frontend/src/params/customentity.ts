// Param matcher for custom entities - excludes reserved routes
const RESERVED_ROUTES = ['contacts', 'accounts', 'admin', 'settings', 'tasks', 'services', 'accept-invite', 'login', 'register'];

export function match(param: string): boolean {
	return !RESERVED_ROUTES.includes(param.toLowerCase());
}
