const PUBLIC_API_URL = "/api/v1";
const API_BASE = PUBLIC_API_URL;
const STORAGE_KEY = "quantico_auth";
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
  if (typeof window === "undefined") return null;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const data = JSON.parse(stored);
      return data.accessToken || null;
    }
  } catch {
  }
  return null;
}
function handleSessionExpired() {
  if (typeof window === "undefined") return;
  localStorage.removeItem(STORAGE_KEY);
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
  const config = {
    method,
    headers: {
      "Content-Type": "application/json",
      ...authHeaders,
      ...headers
    },
    signal
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
        if (response.status === 401 && errorMessage.toLowerCase().includes("invalid or expired token")) {
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
