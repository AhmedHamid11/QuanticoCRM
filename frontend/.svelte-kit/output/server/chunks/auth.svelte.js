import "clsx";
const STORAGE_KEY = "quantico_auth";
function getInitialState() {
  if (typeof window === "undefined") {
    return {
      user: null,
      currentOrg: null,
      accessToken: null,
      // Memory only - NOT restored from storage
      expiresAt: null,
      isAuthenticated: false,
      isLoading: true,
      // Start true - we'll try silent refresh
      isImpersonation: false,
      impersonatedBy: null,
      mustChangePassword: false
    };
  }
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const data = JSON.parse(stored);
      return {
        user: data.user,
        currentOrg: data.currentOrg,
        accessToken: null,
        // DO NOT restore - memory only
        expiresAt: null,
        // Must re-validate via refresh
        isAuthenticated: false,
        // Not authenticated until refresh succeeds
        isLoading: true,
        // Will attempt silent refresh
        isImpersonation: data.isImpersonation || false,
        impersonatedBy: data.impersonatedBy || null,
        mustChangePassword: false
        // Will be set on login/refresh
      };
    }
  } catch (e) {
    console.error("Failed to restore auth state:", e);
  }
  return {
    user: null,
    currentOrg: null,
    accessToken: null,
    expiresAt: null,
    isAuthenticated: false,
    isLoading: false,
    isImpersonation: false,
    impersonatedBy: null,
    mustChangePassword: false
  };
}
let state = getInitialState();
const auth = {
  get user() {
    return state.user;
  },
  get currentOrg() {
    return state.currentOrg;
  },
  get accessToken() {
    return state.accessToken;
  },
  get isAuthenticated() {
    return state.isAuthenticated;
  },
  get isLoading() {
    return state.isLoading;
  },
  get isPlatformAdmin() {
    return state.user?.isPlatformAdmin ?? false;
  },
  get isImpersonation() {
    return state.isImpersonation;
  },
  get memberships() {
    return state.user?.memberships ?? [];
  },
  get isAdmin() {
    const role = state.currentOrg?.role;
    return role === "admin" || role === "owner";
  },
  get canAccessSetup() {
    return state.user?.isPlatformAdmin || state.isImpersonation || this.isAdmin;
  }
};
export {
  auth as a
};
