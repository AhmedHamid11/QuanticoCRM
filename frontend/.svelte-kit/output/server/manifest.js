export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.BZtM1Hu6.js",app:"_app/immutable/entry/app.DZkOVN1e.js",imports:["_app/immutable/entry/start.BZtM1Hu6.js","_app/immutable/chunks/AvL7AfvN.js","_app/immutable/chunks/rM-kmhrg.js","_app/immutable/chunks/Dby6eIWP.js","_app/immutable/chunks/2V7oVLvT.js","_app/immutable/chunks/W5bDyCTn.js","_app/immutable/entry/app.DZkOVN1e.js","_app/immutable/chunks/C1FmrZbK.js","_app/immutable/chunks/rM-kmhrg.js","_app/immutable/chunks/DKnnrXEH.js","_app/immutable/chunks/BogfxaM7.js","_app/immutable/chunks/W5bDyCTn.js","_app/immutable/chunks/BW-32thp.js","_app/immutable/chunks/DxXcKG-N.js","_app/immutable/chunks/Dolm3HPI.js","_app/immutable/chunks/5tiuYX7v.js","_app/immutable/chunks/DqaBWTkN.js","_app/immutable/chunks/Dby6eIWP.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js')),
			__memo(() => import('./nodes/3.js')),
			__memo(() => import('./nodes/4.js')),
			__memo(() => import('./nodes/5.js')),
			__memo(() => import('./nodes/6.js')),
			__memo(() => import('./nodes/7.js')),
			__memo(() => import('./nodes/8.js')),
			__memo(() => import('./nodes/9.js')),
			__memo(() => import('./nodes/10.js')),
			__memo(() => import('./nodes/11.js')),
			__memo(() => import('./nodes/12.js')),
			__memo(() => import('./nodes/13.js')),
			__memo(() => import('./nodes/14.js')),
			__memo(() => import('./nodes/15.js')),
			__memo(() => import('./nodes/16.js')),
			__memo(() => import('./nodes/17.js')),
			__memo(() => import('./nodes/18.js')),
			__memo(() => import('./nodes/19.js')),
			__memo(() => import('./nodes/20.js')),
			__memo(() => import('./nodes/21.js')),
			__memo(() => import('./nodes/22.js')),
			__memo(() => import('./nodes/23.js')),
			__memo(() => import('./nodes/24.js')),
			__memo(() => import('./nodes/25.js')),
			__memo(() => import('./nodes/26.js')),
			__memo(() => import('./nodes/27.js')),
			__memo(() => import('./nodes/28.js')),
			__memo(() => import('./nodes/29.js')),
			__memo(() => import('./nodes/30.js')),
			__memo(() => import('./nodes/31.js')),
			__memo(() => import('./nodes/32.js')),
			__memo(() => import('./nodes/33.js')),
			__memo(() => import('./nodes/34.js')),
			__memo(() => import('./nodes/35.js')),
			__memo(() => import('./nodes/36.js')),
			__memo(() => import('./nodes/37.js')),
			__memo(() => import('./nodes/38.js')),
			__memo(() => import('./nodes/39.js')),
			__memo(() => import('./nodes/40.js')),
			__memo(() => import('./nodes/41.js')),
			__memo(() => import('./nodes/42.js')),
			__memo(() => import('./nodes/43.js')),
			__memo(() => import('./nodes/44.js')),
			__memo(() => import('./nodes/45.js')),
			__memo(() => import('./nodes/46.js')),
			__memo(() => import('./nodes/47.js')),
			__memo(() => import('./nodes/48.js')),
			__memo(() => import('./nodes/49.js')),
			__memo(() => import('./nodes/50.js')),
			__memo(() => import('./nodes/51.js')),
			__memo(() => import('./nodes/52.js')),
			__memo(() => import('./nodes/53.js')),
			__memo(() => import('./nodes/54.js')),
			__memo(() => import('./nodes/55.js')),
			__memo(() => import('./nodes/56.js')),
			__memo(() => import('./nodes/57.js')),
			__memo(() => import('./nodes/58.js')),
			__memo(() => import('./nodes/59.js')),
			__memo(() => import('./nodes/60.js')),
			__memo(() => import('./nodes/61.js')),
			__memo(() => import('./nodes/62.js')),
			__memo(() => import('./nodes/63.js')),
			__memo(() => import('./nodes/64.js')),
			__memo(() => import('./nodes/65.js')),
			__memo(() => import('./nodes/66.js')),
			__memo(() => import('./nodes/67.js')),
			__memo(() => import('./nodes/68.js')),
			__memo(() => import('./nodes/69.js')),
			__memo(() => import('./nodes/70.js')),
			__memo(() => import('./nodes/71.js')),
			__memo(() => import('./nodes/72.js')),
			__memo(() => import('./nodes/73.js')),
			__memo(() => import('./nodes/74.js')),
			__memo(() => import('./nodes/75.js')),
			__memo(() => import('./nodes/76.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 6 },
				endpoint: null
			},
			{
				id: "/accept-invite",
				pattern: /^\/accept-invite\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 15 },
				endpoint: null
			},
			{
				id: "/accounts",
				pattern: /^\/accounts\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 16 },
				endpoint: null
			},
			{
				id: "/accounts/new",
				pattern: /^\/accounts\/new\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 19 },
				endpoint: null
			},
			{
				id: "/accounts/[id]",
				pattern: /^\/accounts\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 17 },
				endpoint: null
			},
			{
				id: "/accounts/[id]/edit",
				pattern: /^\/accounts\/([^/]+?)\/edit\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 18 },
				endpoint: null
			},
			{
				id: "/admin",
				pattern: /^\/admin\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 20 },
				endpoint: null
			},
			{
				id: "/admin/api-tokens",
				pattern: /^\/admin\/api-tokens\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 21 },
				endpoint: null
			},
			{
				id: "/admin/audit-logs",
				pattern: /^\/admin\/audit-logs\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 22 },
				endpoint: null
			},
			{
				id: "/admin/changelog",
				pattern: /^\/admin\/changelog\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 23 },
				endpoint: null
			},
			{
				id: "/admin/data-explorer",
				pattern: /^\/admin\/data-explorer\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 24 },
				endpoint: null
			},
			{
				id: "/admin/data-quality",
				pattern: /^\/admin\/data-quality\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 25 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/duplicate-rules",
				pattern: /^\/admin\/data-quality\/duplicate-rules\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 26 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/merge-history",
				pattern: /^\/admin\/data-quality\/merge-history\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 29 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/merge/history",
				pattern: /^\/admin\/data-quality\/merge\/history\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 28 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/merge/[groupId]",
				pattern: /^\/admin\/data-quality\/merge\/([^/]+?)\/?$/,
				params: [{"name":"groupId","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 27 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/review-queue",
				pattern: /^\/admin\/data-quality\/review-queue\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 30 },
				endpoint: null
			},
			{
				id: "/admin/data-quality/scan-jobs",
				pattern: /^\/admin\/data-quality\/scan-jobs\/?$/,
				params: [],
				page: { layouts: [0,3,5,], errors: [1,4,,], leaf: 31 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager",
				pattern: /^\/admin\/entity-manager\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 32 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 33 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/bearings",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/bearings\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 34 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/fields",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/fields\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 35 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/layouts",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/layouts\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 36 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/layouts/[type]",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/layouts\/([^/]+?)\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false},{"name":"type","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 37 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/related-lists",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/related-lists\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 38 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/validation-rules",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/validation-rules\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 39 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/validation-rules/new",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/validation-rules\/new\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 41 },
				endpoint: null
			},
			{
				id: "/admin/entity-manager/[entity]/validation-rules/[id]",
				pattern: /^\/admin\/entity-manager\/([^/]+?)\/validation-rules\/([^/]+?)\/?$/,
				params: [{"name":"entity","optional":false,"rest":false,"chained":false},{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 40 },
				endpoint: null
			},
			{
				id: "/admin/flows",
				pattern: /^\/admin\/flows\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 42 },
				endpoint: null
			},
			{
				id: "/admin/flows/new",
				pattern: /^\/admin\/flows\/new\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 44 },
				endpoint: null
			},
			{
				id: "/admin/flows/[id]",
				pattern: /^\/admin\/flows\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 43 },
				endpoint: null
			},
			{
				id: "/admin/import",
				pattern: /^\/admin\/import\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 45 },
				endpoint: null
			},
			{
				id: "/admin/integrations",
				pattern: /^\/admin\/integrations\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 46 },
				endpoint: null
			},
			{
				id: "/admin/integrations/salesforce",
				pattern: /^\/admin\/integrations\/salesforce\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 47 },
				endpoint: null
			},
			{
				id: "/admin/mirrors",
				pattern: /^\/admin\/mirrors\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 48 },
				endpoint: null
			},
			{
				id: "/admin/mirrors/[id]",
				pattern: /^\/admin\/mirrors\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 49 },
				endpoint: null
			},
			{
				id: "/admin/navigation",
				pattern: /^\/admin\/navigation\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 50 },
				endpoint: null
			},
			{
				id: "/admin/pages",
				pattern: /^\/admin\/pages\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 51 },
				endpoint: null
			},
			{
				id: "/admin/pages/[id]",
				pattern: /^\/admin\/pages\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 52 },
				endpoint: null
			},
			{
				id: "/admin/pdf-templates",
				pattern: /^\/admin\/pdf-templates\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 53 },
				endpoint: null
			},
			{
				id: "/admin/pdf-templates/[id]",
				pattern: /^\/admin\/pdf-templates\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 54 },
				endpoint: null
			},
			{
				id: "/admin/platform",
				pattern: /^\/admin\/platform\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 55 },
				endpoint: null
			},
			{
				id: "/admin/settings",
				pattern: /^\/admin\/settings\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 56 },
				endpoint: null
			},
			{
				id: "/admin/settings/webhooks",
				pattern: /^\/admin\/settings\/webhooks\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 57 },
				endpoint: null
			},
			{
				id: "/admin/tripwires",
				pattern: /^\/admin\/tripwires\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 58 },
				endpoint: null
			},
			{
				id: "/admin/tripwires/new",
				pattern: /^\/admin\/tripwires\/new\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 61 },
				endpoint: null
			},
			{
				id: "/admin/tripwires/[id]",
				pattern: /^\/admin\/tripwires\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 59 },
				endpoint: null
			},
			{
				id: "/admin/tripwires/[id]/logs",
				pattern: /^\/admin\/tripwires\/([^/]+?)\/logs\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 60 },
				endpoint: null
			},
			{
				id: "/admin/users",
				pattern: /^\/admin\/users\/?$/,
				params: [],
				page: { layouts: [0,3,], errors: [1,4,], leaf: 62 },
				endpoint: null
			},
			{
				id: "/contacts",
				pattern: /^\/contacts\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 63 },
				endpoint: null
			},
			{
				id: "/contacts/new",
				pattern: /^\/contacts\/new\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 66 },
				endpoint: null
			},
			{
				id: "/contacts/[id]",
				pattern: /^\/contacts\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 64 },
				endpoint: null
			},
			{
				id: "/contacts/[id]/edit",
				pattern: /^\/contacts\/([^/]+?)\/edit\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 65 },
				endpoint: null
			},
			{
				id: "/(auth)/forgot-password",
				pattern: /^\/forgot-password\/?$/,
				params: [],
				page: { layouts: [0,2,], errors: [1,,], leaf: 7 },
				endpoint: null
			},
			{
				id: "/(auth)/login",
				pattern: /^\/login\/?$/,
				params: [],
				page: { layouts: [0,2,], errors: [1,,], leaf: 8 },
				endpoint: null
			},
			{
				id: "/p/[slug]",
				pattern: /^\/p\/([^/]+?)\/?$/,
				params: [{"name":"slug","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 67 },
				endpoint: null
			},
			{
				id: "/quotes",
				pattern: /^\/quotes\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 68 },
				endpoint: null
			},
			{
				id: "/quotes/new",
				pattern: /^\/quotes\/new\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 71 },
				endpoint: null
			},
			{
				id: "/quotes/[id]",
				pattern: /^\/quotes\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 69 },
				endpoint: null
			},
			{
				id: "/quotes/[id]/edit",
				pattern: /^\/quotes\/([^/]+?)\/edit\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 70 },
				endpoint: null
			},
			{
				id: "/(auth)/register",
				pattern: /^\/register\/?$/,
				params: [],
				page: { layouts: [0,2,], errors: [1,,], leaf: 9 },
				endpoint: null
			},
			{
				id: "/(auth)/reset-password",
				pattern: /^\/reset-password\/?$/,
				params: [],
				page: { layouts: [0,2,], errors: [1,,], leaf: 10 },
				endpoint: null
			},
			{
				id: "/settings/profile",
				pattern: /^\/settings\/profile\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 72 },
				endpoint: null
			},
			{
				id: "/tasks",
				pattern: /^\/tasks\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 73 },
				endpoint: null
			},
			{
				id: "/tasks/new",
				pattern: /^\/tasks\/new\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 76 },
				endpoint: null
			},
			{
				id: "/tasks/[id]",
				pattern: /^\/tasks\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 74 },
				endpoint: null
			},
			{
				id: "/tasks/[id]/edit",
				pattern: /^\/tasks\/([^/]+?)\/edit\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 75 },
				endpoint: null
			},
			{
				id: "/[entity=customentity]",
				pattern: /^\/([^/]+?)\/?$/,
				params: [{"name":"entity","matcher":"customentity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 11 },
				endpoint: null
			},
			{
				id: "/[entity=customentity]/new",
				pattern: /^\/([^/]+?)\/new\/?$/,
				params: [{"name":"entity","matcher":"customentity","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 14 },
				endpoint: null
			},
			{
				id: "/[entity=customentity]/[id]",
				pattern: /^\/([^/]+?)\/([^/]+?)\/?$/,
				params: [{"name":"entity","matcher":"customentity","optional":false,"rest":false,"chained":false},{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 12 },
				endpoint: null
			},
			{
				id: "/[entity=customentity]/[id]/edit",
				pattern: /^\/([^/]+?)\/([^/]+?)\/edit\/?$/,
				params: [{"name":"entity","matcher":"customentity","optional":false,"rest":false,"chained":false},{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 13 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			const { match: customentity } = await import ('./entries/matchers/customentity.js')
			return { customentity };
		},
		server_assets: {}
	}
}
})();
