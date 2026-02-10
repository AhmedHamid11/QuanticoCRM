import "clsx";
const PUBLIC_API_URL = "/api/v1";
const STORAGE_KEY = "quantico_auth";
let refreshPromise = null;
const REFRESH_LOCK_KEY = "quantico_refresh_lock";
const LOCK_TTL_MS = 1e4;
function isRefreshLockedByOtherTab() {
  if (typeof window === "undefined") return false;
  try {
    const lock = localStorage.getItem(REFRESH_LOCK_KEY);
    if (!lock) return false;
    return Date.now() - parseInt(lock, 10) < LOCK_TTL_MS;
  } catch {
    return false;
  }
}
function setRefreshLock() {
  try {
    localStorage.setItem(REFRESH_LOCK_KEY, Date.now().toString());
  } catch {
  }
}
function clearRefreshLock() {
  try {
    localStorage.removeItem(REFRESH_LOCK_KEY);
  } catch {
  }
}
function waitForRefreshLock() {
  return new Promise((resolve) => {
    const maxWait = setTimeout(resolve, LOCK_TTL_MS);
    const interval = setInterval(
      () => {
        if (!isRefreshLockedByOtherTab()) {
          clearInterval(interval);
          clearTimeout(maxWait);
          resolve();
        }
      },
      100
    );
  });
}
async function silentRefresh() {
  if (refreshPromise) {
    return refreshPromise;
  }
  if (isRefreshLockedByOtherTab()) {
    await waitForRefreshLock();
  }
  refreshPromise = (async () => {
    setRefreshLock();
    try {
      return await doSilentRefresh();
    } finally {
      clearRefreshLock();
    }
  })();
  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}
async function doSilentRefresh() {
  try {
    const API_BASE = PUBLIC_API_URL || "/api/v1";
    const response = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include"
      // CRITICAL: Sends HttpOnly cookie
    });
    if (response.ok) {
      const data = await response.json();
      state.accessToken = data.accessToken;
      state.expiresAt = new Date(data.expiresAt);
      state.isAuthenticated = true;
      if (data.user) {
        state.user = data.user;
        const defaultOrg = data.user.memberships.find((m) => m.isDefault) || data.user.memberships[0];
        state.currentOrg = defaultOrg || null;
      }
      const claims = decodeJWT(data.accessToken);
      state.mustChangePassword = claims?.mustChangePassword === true;
      persistState();
      return true;
    }
    return false;
  } catch {
    return false;
  }
}
function getInitialState() {
  if (typeof window === "undefined") {
    return {
      user: null,
      currentOrg: null,
      accessToken: null,
      // Memory only - NOT restored from storage
      refreshToken: null,
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
        refreshToken: null,
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
    refreshToken: null,
    expiresAt: null,
    isAuthenticated: false,
    isLoading: false,
    isImpersonation: false,
    impersonatedBy: null,
    mustChangePassword: false
  };
}
let state = getInitialState();
function persistState() {
  if (typeof window === "undefined") return;
  try {
    const toStore = {
      user: state.user,
      currentOrg: state.currentOrg,
      isImpersonation: state.isImpersonation,
      impersonatedBy: state.impersonatedBy
      // NO tokens persisted
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
  } catch (e) {
    console.error("Failed to persist auth state:", e);
  }
}
function decodeJWT(token) {
  try {
    const base64Url = token.split(".")[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(atob(base64).split("").map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2)).join(""));
    return JSON.parse(jsonPayload);
  } catch {
    return null;
  }
}
async function refreshTokens() {
  return silentRefresh();
}
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
  // Role helpers for permission checks
  get role() {
    return state.currentOrg?.role ?? "user";
  },
  get isOwner() {
    return state.currentOrg?.role === "owner";
  },
  get isAdmin() {
    const role = state.currentOrg?.role;
    return role === "admin" || role === "owner";
  },
  get canAccessSetup() {
    return state.user?.isPlatformAdmin || state.isImpersonation || this.isAdmin;
  },
  get mustChangePassword() {
    return state.mustChangePassword;
  }
};
export {
  PUBLIC_API_URL as P,
  auth as a,
  refreshTokens as r
};
