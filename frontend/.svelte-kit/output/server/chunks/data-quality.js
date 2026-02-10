import { g as get } from "./api.js";
async function listPendingAlerts(params) {
  const queryParams = new URLSearchParams();
  if (params?.entityType) queryParams.set("entityType", params.entityType);
  if (params?.page) queryParams.set("page", params.page.toString());
  if (params?.pageSize) queryParams.set("pageSize", params.pageSize.toString());
  const query = queryParams.toString() ? `?${queryParams.toString()}` : "";
  return get(`/dedup/pending-alerts${query}`);
}
async function mergeHistory(params) {
  const queryParams = new URLSearchParams();
  if (params?.entityType) queryParams.set("entityType", params.entityType);
  if (params?.page) queryParams.set("page", params.page.toString());
  if (params?.pageSize) queryParams.set("pageSize", params.pageSize.toString());
  const query = queryParams.toString() ? `?${queryParams.toString()}` : "";
  return get(`/merge/history${query}`);
}
export {
  listPendingAlerts as l,
  mergeHistory as m
};
