import type * as Kit from '@sveltejs/kit';

type Expand<T> = T extends infer O ? { [K in keyof O]: O[K] } : never;
// @ts-ignore
type MatcherParam<M> = M extends (param : string) => param is infer U ? U extends string ? U : string : string;
type RouteParams = {  };
type RouteId = '/admin';
type MaybeWithVoid<T> = {} extends T ? T | void : T;
export type RequiredKeys<T> = { [K in keyof T]-?: {} extends { [P in K]: T[K] } ? never : K; }[keyof T];
type OutputDataShape<T> = MaybeWithVoid<Omit<App.PageData, RequiredKeys<T>> & Partial<Pick<App.PageData, keyof T & keyof App.PageData>> & Record<string, any>>
type EnsureDefined<T> = T extends null | undefined ? {} : T;
type OptionalUnion<U extends Record<string, any>, A extends keyof U = U extends U ? keyof U : never> = U extends unknown ? { [P in Exclude<A, keyof U>]?: never } & U : never;
export type Snapshot<T = any> = Kit.Snapshot<T>;
type PageParentData = Omit<EnsureDefined<import('../$types.js').LayoutData>, keyof LayoutData> & EnsureDefined<LayoutData>;
type LayoutRouteId = RouteId | "/admin" | "/admin/api-tokens" | "/admin/changelog" | "/admin/data-explorer" | "/admin/entity-manager" | "/admin/entity-manager/[entity]" | "/admin/entity-manager/[entity]/bearings" | "/admin/entity-manager/[entity]/fields" | "/admin/entity-manager/[entity]/layouts" | "/admin/entity-manager/[entity]/layouts/[type]" | "/admin/entity-manager/[entity]/related-lists" | "/admin/entity-manager/[entity]/validation-rules" | "/admin/entity-manager/[entity]/validation-rules/[id]" | "/admin/entity-manager/[entity]/validation-rules/new" | "/admin/flows" | "/admin/flows/[id]" | "/admin/flows/new" | "/admin/navigation" | "/admin/pages" | "/admin/pages/[id]" | "/admin/pdf-templates" | "/admin/pdf-templates/[id]" | "/admin/platform" | "/admin/settings" | "/admin/settings/webhooks" | "/admin/tripwires" | "/admin/tripwires/[id]" | "/admin/tripwires/[id]/logs" | "/admin/tripwires/new" | "/admin/users"
type LayoutParams = RouteParams & { entity?: string; type?: string; id?: string }
type LayoutParentData = EnsureDefined<import('../$types.js').LayoutData>;

export type PageServerData = null;
export type PageData = Expand<PageParentData>;
export type PageProps = { params: RouteParams; data: PageData }
export type LayoutServerData = null;
export type LayoutData = Expand<LayoutParentData>;
export type LayoutProps = { params: LayoutParams; data: LayoutData; children: import("svelte").Snippet }