import { P as PUBLIC_API_URL, r as refreshTokens, a as auth } from "./auth.svelte.js";
const API_BASE = PUBLIC_API_URL;
class ApiError extends Error {
  fieldErrors;
  status;
  constructor(message, status, fieldErrors) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fieldErrors = fieldErrors;
  }
}
function getAuthToken() {
  return auth.accessToken;
}
function getCSRFToken() {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}
function handleSessionExpired() {
  if (typeof window === "undefined") return;
  window.location.href = "/login?session_expired=true";
}
function isRetryableError(error) {
  if (error instanceof Error) {
    const message = error.message.toLowerCase();
    return message.includes("failed to fetch") || message.includes("network") || message.includes("cors") || message.includes("err_failed");
  }
  return false;
}
function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
async function api(endpoint, options = {}) {
  const { method = "GET", body, headers = {}, signal } = options;
  const maxRetries = 3;
  const baseDelay = 200;
  const authToken = getAuthToken();
  const authHeaders = {};
  if (authToken) {
    authHeaders["Authorization"] = `Bearer ${authToken}`;
  }
  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    const csrfToken = getCSRFToken();
    if (csrfToken) {
      authHeaders["X-CSRF-Token"] = csrfToken;
    }
  }
  const config = {
    method,
    headers: {
      "Content-Type": "application/json",
      ...authHeaders,
      ...headers
    },
    signal,
    credentials: "include"
    // Send cookies for CSRF
  };
  if (body) {
    config.body = JSON.stringify(body);
  }
  let lastError = null;
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      if (signal?.aborted) {
        throw new DOMException("Aborted", "AbortError");
      }
      const response = await fetch(`${API_BASE}${endpoint}`, config);
      if (!response.ok) {
        const error = await response.json().catch(() => ({ error: "Unknown error" }));
        const errorMessage = error.error || `HTTP ${response.status}`;
        if (response.status === 401) {
          const refreshed = await refreshTokens();
          if (refreshed) {
            const retryHeaders = {
              "Content-Type": "application/json",
              ...headers
            };
            const newToken = auth.accessToken;
            if (newToken) {
              retryHeaders["Authorization"] = `Bearer ${newToken}`;
            }
            if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
              const freshCsrf = getCSRFToken();
              if (freshCsrf) {
                retryHeaders["X-CSRF-Token"] = freshCsrf;
              }
            }
            const retryResponse = await fetch(`${API_BASE}${endpoint}`, {
              method,
              headers: retryHeaders,
              body: body ? JSON.stringify(body) : void 0,
              signal,
              credentials: "include"
            });
            if (retryResponse.ok) {
              if (retryResponse.status === 204) return void 0;
              return retryResponse.json();
            }
          }
          handleSessionExpired();
          throw new ApiError("Session expired", 401);
        }
        if (response.status === 422 && error.fieldErrors) {
          throw new ApiError(errorMessage, 422, error.fieldErrors);
        }
        if (response.status >= 400 && response.status < 500) {
          throw new ApiError(errorMessage, response.status);
        }
        if (response.status >= 500 && attempt < maxRetries - 1) {
          lastError = new ApiError(errorMessage, response.status);
          await delay(baseDelay * Math.pow(2, attempt));
          continue;
        }
        throw new ApiError(errorMessage, response.status);
      }
      if (response.status === 204) {
        return void 0;
      }
      return response.json();
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        throw error;
      }
      if (error instanceof ApiError) {
        throw error;
      }
      if (isRetryableError(error) && attempt < maxRetries - 1) {
        lastError = error;
        await delay(baseDelay * Math.pow(2, attempt));
        continue;
      }
      throw error;
    }
  }
  throw lastError || new Error("Request failed after retries");
}
const get = (endpoint, signal) => api(endpoint, { signal });
export {
  get as g
};
