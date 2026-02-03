import "clsx";
const STORAGE_KEY = "quantico_auth";
function getInitialState() {
  if (typeof window === "undefined") {
    return {
      user: null,
      currentOrg: null,
      accessToken: null,
      refreshToken: null,
      expiresAt: null,
      isAuthenticated: false,
      isLoading: true,
      isImpersonation: false,
      impersonatedBy: null
    };
  }
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const data = JSON.parse(stored);
      return {
        ...data,
        expiresAt: data.expiresAt ? new Date(data.expiresAt) : null,
        isLoading: false,
        isAuthenticated: !!data.accessToken
      };
    }
  } catch (e) {
    console.error("Failed to restore auth state:", e);
  }
  return {
    user: null,
    currentOrg: null,
    accessToken: null,
    refreshToken: null,
    expiresAt: null,
    isAuthenticated: false,
    isLoading: false,
    isImpersonation: false,
    impersonatedBy: null
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
    return state.user?.isPlatformAdmin || this.isAdmin;
  }
};
export {
  auth as a
};
