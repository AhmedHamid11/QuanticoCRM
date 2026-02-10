
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	export interface AppTypes {
		RouteId(): "/(auth)" | "/" | "/accept-invite" | "/accounts" | "/accounts/new" | "/accounts/[id]" | "/accounts/[id]/edit" | "/admin" | "/admin/api-tokens" | "/admin/audit-logs" | "/admin/changelog" | "/admin/data-explorer" | "/admin/data-quality" | "/admin/data-quality/duplicate-rules" | "/admin/data-quality/merge-history" | "/admin/data-quality/merge" | "/admin/data-quality/merge/history" | "/admin/data-quality/merge/[groupId]" | "/admin/data-quality/review-queue" | "/admin/data-quality/scan-jobs" | "/admin/entity-manager" | "/admin/entity-manager/[entity]" | "/admin/entity-manager/[entity]/bearings" | "/admin/entity-manager/[entity]/fields" | "/admin/entity-manager/[entity]/layouts" | "/admin/entity-manager/[entity]/layouts/[type]" | "/admin/entity-manager/[entity]/related-lists" | "/admin/entity-manager/[entity]/validation-rules" | "/admin/entity-manager/[entity]/validation-rules/new" | "/admin/entity-manager/[entity]/validation-rules/[id]" | "/admin/flows" | "/admin/flows/new" | "/admin/flows/[id]" | "/admin/import" | "/admin/integrations" | "/admin/integrations/salesforce" | "/admin/mirrors" | "/admin/mirrors/[id]" | "/admin/navigation" | "/admin/pages" | "/admin/pages/[id]" | "/admin/pdf-templates" | "/admin/pdf-templates/[id]" | "/admin/platform" | "/admin/settings" | "/admin/settings/webhooks" | "/admin/tripwires" | "/admin/tripwires/new" | "/admin/tripwires/[id]" | "/admin/tripwires/[id]/logs" | "/admin/users" | "/contacts" | "/contacts/new" | "/contacts/[id]" | "/contacts/[id]/edit" | "/(auth)/forgot-password" | "/(auth)/login" | "/p" | "/p/[slug]" | "/quotes" | "/quotes/new" | "/quotes/[id]" | "/quotes/[id]/edit" | "/(auth)/register" | "/(auth)/reset-password" | "/settings" | "/settings/profile" | "/tasks" | "/tasks/new" | "/tasks/[id]" | "/tasks/[id]/edit" | "/[entity=customentity]" | "/[entity=customentity]/new" | "/[entity=customentity]/[id]" | "/[entity=customentity]/[id]/edit";
		RouteParams(): {
			"/accounts/[id]": { id: string };
			"/accounts/[id]/edit": { id: string };
			"/admin/data-quality/merge/[groupId]": { groupId: string };
			"/admin/entity-manager/[entity]": { entity: string };
			"/admin/entity-manager/[entity]/bearings": { entity: string };
			"/admin/entity-manager/[entity]/fields": { entity: string };
			"/admin/entity-manager/[entity]/layouts": { entity: string };
			"/admin/entity-manager/[entity]/layouts/[type]": { entity: string; type: string };
			"/admin/entity-manager/[entity]/related-lists": { entity: string };
			"/admin/entity-manager/[entity]/validation-rules": { entity: string };
			"/admin/entity-manager/[entity]/validation-rules/new": { entity: string };
			"/admin/entity-manager/[entity]/validation-rules/[id]": { entity: string; id: string };
			"/admin/flows/[id]": { id: string };
			"/admin/mirrors/[id]": { id: string };
			"/admin/pages/[id]": { id: string };
			"/admin/pdf-templates/[id]": { id: string };
			"/admin/tripwires/[id]": { id: string };
			"/admin/tripwires/[id]/logs": { id: string };
			"/contacts/[id]": { id: string };
			"/contacts/[id]/edit": { id: string };
			"/p/[slug]": { slug: string };
			"/quotes/[id]": { id: string };
			"/quotes/[id]/edit": { id: string };
			"/tasks/[id]": { id: string };
			"/tasks/[id]/edit": { id: string };
			"/[entity=customentity]": { entity: string };
			"/[entity=customentity]/new": { entity: string };
			"/[entity=customentity]/[id]": { entity: string; id: string };
			"/[entity=customentity]/[id]/edit": { entity: string; id: string }
		};
		LayoutParams(): {
			"/(auth)": Record<string, never>;
			"/": { id?: string; groupId?: string; entity?: string; type?: string; slug?: string };
			"/accept-invite": Record<string, never>;
			"/accounts": { id?: string };
			"/accounts/new": Record<string, never>;
			"/accounts/[id]": { id: string };
			"/accounts/[id]/edit": { id: string };
			"/admin": { groupId?: string; entity?: string; type?: string; id?: string };
			"/admin/api-tokens": Record<string, never>;
			"/admin/audit-logs": Record<string, never>;
			"/admin/changelog": Record<string, never>;
			"/admin/data-explorer": Record<string, never>;
			"/admin/data-quality": { groupId?: string };
			"/admin/data-quality/duplicate-rules": Record<string, never>;
			"/admin/data-quality/merge-history": Record<string, never>;
			"/admin/data-quality/merge": { groupId?: string };
			"/admin/data-quality/merge/history": Record<string, never>;
			"/admin/data-quality/merge/[groupId]": { groupId: string };
			"/admin/data-quality/review-queue": Record<string, never>;
			"/admin/data-quality/scan-jobs": Record<string, never>;
			"/admin/entity-manager": { entity?: string; type?: string; id?: string };
			"/admin/entity-manager/[entity]": { entity: string; type?: string; id?: string };
			"/admin/entity-manager/[entity]/bearings": { entity: string };
			"/admin/entity-manager/[entity]/fields": { entity: string };
			"/admin/entity-manager/[entity]/layouts": { entity: string; type?: string };
			"/admin/entity-manager/[entity]/layouts/[type]": { entity: string; type: string };
			"/admin/entity-manager/[entity]/related-lists": { entity: string };
			"/admin/entity-manager/[entity]/validation-rules": { entity: string; id?: string };
			"/admin/entity-manager/[entity]/validation-rules/new": { entity: string };
			"/admin/entity-manager/[entity]/validation-rules/[id]": { entity: string; id: string };
			"/admin/flows": { id?: string };
			"/admin/flows/new": Record<string, never>;
			"/admin/flows/[id]": { id: string };
			"/admin/import": Record<string, never>;
			"/admin/integrations": Record<string, never>;
			"/admin/integrations/salesforce": Record<string, never>;
			"/admin/mirrors": { id?: string };
			"/admin/mirrors/[id]": { id: string };
			"/admin/navigation": Record<string, never>;
			"/admin/pages": { id?: string };
			"/admin/pages/[id]": { id: string };
			"/admin/pdf-templates": { id?: string };
			"/admin/pdf-templates/[id]": { id: string };
			"/admin/platform": Record<string, never>;
			"/admin/settings": Record<string, never>;
			"/admin/settings/webhooks": Record<string, never>;
			"/admin/tripwires": { id?: string };
			"/admin/tripwires/new": Record<string, never>;
			"/admin/tripwires/[id]": { id: string };
			"/admin/tripwires/[id]/logs": { id: string };
			"/admin/users": Record<string, never>;
			"/contacts": { id?: string };
			"/contacts/new": Record<string, never>;
			"/contacts/[id]": { id: string };
			"/contacts/[id]/edit": { id: string };
			"/(auth)/forgot-password": Record<string, never>;
			"/(auth)/login": Record<string, never>;
			"/p": { slug?: string };
			"/p/[slug]": { slug: string };
			"/quotes": { id?: string };
			"/quotes/new": Record<string, never>;
			"/quotes/[id]": { id: string };
			"/quotes/[id]/edit": { id: string };
			"/(auth)/register": Record<string, never>;
			"/(auth)/reset-password": Record<string, never>;
			"/settings": Record<string, never>;
			"/settings/profile": Record<string, never>;
			"/tasks": { id?: string };
			"/tasks/new": Record<string, never>;
			"/tasks/[id]": { id: string };
			"/tasks/[id]/edit": { id: string };
			"/[entity=customentity]": { entity: string; id?: string };
			"/[entity=customentity]/new": { entity: string };
			"/[entity=customentity]/[id]": { entity: string; id: string };
			"/[entity=customentity]/[id]/edit": { entity: string; id: string }
		};
		Pathname(): "/" | "/accept-invite" | "/accept-invite/" | "/accounts" | "/accounts/" | "/accounts/new" | "/accounts/new/" | `/accounts/${string}` & {} | `/accounts/${string}/` & {} | `/accounts/${string}/edit` & {} | `/accounts/${string}/edit/` & {} | "/admin" | "/admin/" | "/admin/api-tokens" | "/admin/api-tokens/" | "/admin/audit-logs" | "/admin/audit-logs/" | "/admin/changelog" | "/admin/changelog/" | "/admin/data-explorer" | "/admin/data-explorer/" | "/admin/data-quality" | "/admin/data-quality/" | "/admin/data-quality/duplicate-rules" | "/admin/data-quality/duplicate-rules/" | "/admin/data-quality/merge-history" | "/admin/data-quality/merge-history/" | "/admin/data-quality/merge" | "/admin/data-quality/merge/" | "/admin/data-quality/merge/history" | "/admin/data-quality/merge/history/" | `/admin/data-quality/merge/${string}` & {} | `/admin/data-quality/merge/${string}/` & {} | "/admin/data-quality/review-queue" | "/admin/data-quality/review-queue/" | "/admin/data-quality/scan-jobs" | "/admin/data-quality/scan-jobs/" | "/admin/entity-manager" | "/admin/entity-manager/" | `/admin/entity-manager/${string}` & {} | `/admin/entity-manager/${string}/` & {} | `/admin/entity-manager/${string}/bearings` & {} | `/admin/entity-manager/${string}/bearings/` & {} | `/admin/entity-manager/${string}/fields` & {} | `/admin/entity-manager/${string}/fields/` & {} | `/admin/entity-manager/${string}/layouts` & {} | `/admin/entity-manager/${string}/layouts/` & {} | `/admin/entity-manager/${string}/layouts/${string}` & {} | `/admin/entity-manager/${string}/layouts/${string}/` & {} | `/admin/entity-manager/${string}/related-lists` & {} | `/admin/entity-manager/${string}/related-lists/` & {} | `/admin/entity-manager/${string}/validation-rules` & {} | `/admin/entity-manager/${string}/validation-rules/` & {} | `/admin/entity-manager/${string}/validation-rules/new` & {} | `/admin/entity-manager/${string}/validation-rules/new/` & {} | `/admin/entity-manager/${string}/validation-rules/${string}` & {} | `/admin/entity-manager/${string}/validation-rules/${string}/` & {} | "/admin/flows" | "/admin/flows/" | "/admin/flows/new" | "/admin/flows/new/" | `/admin/flows/${string}` & {} | `/admin/flows/${string}/` & {} | "/admin/import" | "/admin/import/" | "/admin/integrations" | "/admin/integrations/" | "/admin/integrations/salesforce" | "/admin/integrations/salesforce/" | "/admin/mirrors" | "/admin/mirrors/" | `/admin/mirrors/${string}` & {} | `/admin/mirrors/${string}/` & {} | "/admin/navigation" | "/admin/navigation/" | "/admin/pages" | "/admin/pages/" | `/admin/pages/${string}` & {} | `/admin/pages/${string}/` & {} | "/admin/pdf-templates" | "/admin/pdf-templates/" | `/admin/pdf-templates/${string}` & {} | `/admin/pdf-templates/${string}/` & {} | "/admin/platform" | "/admin/platform/" | "/admin/settings" | "/admin/settings/" | "/admin/settings/webhooks" | "/admin/settings/webhooks/" | "/admin/tripwires" | "/admin/tripwires/" | "/admin/tripwires/new" | "/admin/tripwires/new/" | `/admin/tripwires/${string}` & {} | `/admin/tripwires/${string}/` & {} | `/admin/tripwires/${string}/logs` & {} | `/admin/tripwires/${string}/logs/` & {} | "/admin/users" | "/admin/users/" | "/contacts" | "/contacts/" | "/contacts/new" | "/contacts/new/" | `/contacts/${string}` & {} | `/contacts/${string}/` & {} | `/contacts/${string}/edit` & {} | `/contacts/${string}/edit/` & {} | "/forgot-password" | "/forgot-password/" | "/login" | "/login/" | "/p" | "/p/" | `/p/${string}` & {} | `/p/${string}/` & {} | "/quotes" | "/quotes/" | "/quotes/new" | "/quotes/new/" | `/quotes/${string}` & {} | `/quotes/${string}/` & {} | `/quotes/${string}/edit` & {} | `/quotes/${string}/edit/` & {} | "/register" | "/register/" | "/reset-password" | "/reset-password/" | "/settings" | "/settings/" | "/settings/profile" | "/settings/profile/" | "/tasks" | "/tasks/" | "/tasks/new" | "/tasks/new/" | `/tasks/${string}` & {} | `/tasks/${string}/` & {} | `/tasks/${string}/edit` & {} | `/tasks/${string}/edit/` & {} | `/${string}` & {} | `/${string}/` & {} | `/${string}/new` & {} | `/${string}/new/` & {} | `/${string}/${string}` & {} | `/${string}/${string}/` & {} | `/${string}/${string}/edit` & {} | `/${string}/${string}/edit/` & {};
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): string & {};
	}
}