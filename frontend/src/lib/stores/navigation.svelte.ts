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
    accentColor?: string;
    features?: Record<string, boolean>;
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
        orgSettings = settingsResult;
        tabs = navResult;
        // If API returns empty array, use fallback defaults
        // This handles orgs where navigation_tabs table exists but has no rows
        if (navResult.length === 0) {
            tabs = [
                { id: 'nav_contacts', label: 'Contacts', href: '/contacts', icon: 'users', entityName: 'Contact', sortOrder: 1, isVisible: true, isSystem: true },
                { id: 'nav_accounts', label: 'Accounts', href: '/accounts', icon: 'building', entityName: 'Account', sortOrder: 2, isVisible: true, isSystem: true },
                { id: 'nav_engagement', label: 'Engagement', href: '/engagement/tasks', icon: 'engagement', sortOrder: 5, isVisible: true, isSystem: true },
                { id: 'nav_admin', label: 'Admin', href: '/admin', icon: 'settings', sortOrder: 100, isVisible: true, isSystem: true }
            ];
        }
        // Filter out Engagement tab if cadences feature is not enabled
        if (!isFeatureEnabled('cadences')) {
            tabs = tabs.filter(t => t.id !== 'nav_engagement');
        }
    } catch (e) {
        error = e instanceof Error ? e.message : 'Failed to load navigation';
        // Fallback to default navigation
        tabs = [
            { id: 'nav_contacts', label: 'Contacts', href: '/contacts', icon: 'users', entityName: 'Contact', sortOrder: 1, isVisible: true, isSystem: true },
            { id: 'nav_accounts', label: 'Accounts', href: '/accounts', icon: 'building', entityName: 'Account', sortOrder: 2, isVisible: true, isSystem: true },
            { id: 'nav_engagement', label: 'Engagement', href: '/engagement/tasks', icon: 'engagement', sortOrder: 5, isVisible: true, isSystem: true },
            { id: 'nav_admin', label: 'Admin', href: '/admin', icon: 'settings', sortOrder: 100, isVisible: true, isSystem: true }
        ];
        // Filter out Engagement tab if cadences feature is not enabled
        if (!isFeatureEnabled('cadences')) {
            tabs = tabs.filter(t => t.id !== 'nav_engagement');
        }
    } finally {
        loading = false;
    }
}

// Check if a feature is enabled for the current org
export function isFeatureEnabled(key: string): boolean {
    return orgSettings?.features?.[key] ?? false;
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

// Get accent color (default deep blue #1e40af)
export function getAccentColor(): string {
    return orgSettings?.accentColor || '#1e40af';
}

// Export the load function
export { loadNavigation };
