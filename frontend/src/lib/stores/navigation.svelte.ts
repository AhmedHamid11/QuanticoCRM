import { get } from '$lib/utils/api';

export interface NavigationTab {
    id: string;
    label: string;
    href: string;
    icon: string;
    entityName?: string;
    sortOrder: number;
    isVisible: boolean;
    isSystem: boolean;
}

export interface OrgSettings {
    orgId: string;
    homePage: string;
}

// Navigation state
let tabs = $state<NavigationTab[]>([]);
let orgSettings = $state<OrgSettings | null>(null);
let loading = $state(false);
let error = $state<string | null>(null);

// Load navigation tabs from API
async function loadNavigation() {
    loading = true;
    error = null;
    try {
        // Load navigation tabs and org settings in parallel
        const [navResult, settingsResult] = await Promise.all([
            get<NavigationTab[]>('/navigation'),
            get<OrgSettings>('/settings').catch(() => null)
        ]);
        tabs = navResult;
        // If API returns empty array, use fallback defaults
        // This handles orgs where navigation_tabs table exists but has no rows
        if (navResult.length === 0) {
            tabs = [
                { id: 'nav_contacts', label: 'Contacts', href: '/contacts', icon: 'users', entityName: 'Contact', sortOrder: 1, isVisible: true, isSystem: true },
                { id: 'nav_accounts', label: 'Accounts', href: '/accounts', icon: 'building', entityName: 'Account', sortOrder: 2, isVisible: true, isSystem: true },
                { id: 'nav_admin', label: 'Admin', href: '/admin', icon: 'settings', sortOrder: 100, isVisible: true, isSystem: true }
            ];
        }
        orgSettings = settingsResult;
    } catch (e) {
        error = e instanceof Error ? e.message : 'Failed to load navigation';
        // Fallback to default navigation
        tabs = [
            { id: 'nav_contacts', label: 'Contacts', href: '/contacts', icon: 'users', entityName: 'Contact', sortOrder: 1, isVisible: true, isSystem: true },
            { id: 'nav_accounts', label: 'Accounts', href: '/accounts', icon: 'building', entityName: 'Account', sortOrder: 2, isVisible: true, isSystem: true },
            { id: 'nav_admin', label: 'Admin', href: '/admin', icon: 'settings', sortOrder: 100, isVisible: true, isSystem: true }
        ];
    } finally {
        loading = false;
    }
}

// Export reactive getters
export function getNavigationTabs() {
    return tabs;
}

export function isNavigationLoading() {
    return loading;
}

export function getNavigationError() {
    return error;
}

// Get entity name from URL path (e.g., '/candidates' -> 'Candidate')
export function getEntityNameFromPath(path: string): string | null {
    const normalizedPath = '/' + path.replace(/^\//, '');
    const tab = tabs.find(t => t.href === normalizedPath);
    return tab?.entityName || null;
}

// Get configured homepage (default '/')
export function getHomePage(): string {
    return orgSettings?.homePage || '/';
}

// Get org settings
export function getOrgSettings(): OrgSettings | null {
    return orgSettings;
}

// Export the load function
export { loadNavigation };
