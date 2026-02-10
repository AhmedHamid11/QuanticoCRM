import "clsx";
import "../../../../../chunks/auth.svelte.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let schedules = [];
    let jobs = [];
    let entities = [];
    let runningJobs = /* @__PURE__ */ new Map();
    let jobPage = 1;
    let jobPageSize = 10;
    (() => {
      const scheduledTypes = new Set(schedules.map((s) => s.entityType));
      return entities.filter((e) => !scheduledTypes.has(e.name));
    })();
    (() => {
      return schedules.map((schedule) => {
        const latestJob = jobs.filter((j) => j.entityType === schedule.entityType).sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime())[0];
        const progress = Array.from(runningJobs.values()).find((p) => p.entityType === schedule.entityType);
        const isRunning = progress?.status === "running";
        return { schedule, latestJob, isRunning, progress };
      }).sort((a, b) => {
        if (!a.schedule.nextRunAt) return 1;
        if (!b.schedule.nextRunAt) return -1;
        return new Date(a.schedule.nextRunAt).getTime() - new Date(b.schedule.nextRunAt).getTime();
      });
    })();
    (() => {
      const sorted = [...jobs].sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime());
      return sorted.slice((jobPage - 1) * jobPageSize, jobPage * jobPageSize);
    })();
    $$renderer2.push(`<div class="space-y-6"><div class="flex items-center justify-between"><div><nav class="text-sm text-gray-500 mb-2"><a href="/admin" class="hover:text-gray-700">Administration</a> <span class="mx-2">/</span> <a href="/admin/data-quality" class="hover:text-gray-700">Data Quality</a> <span class="mx-2">/</span> <span class="text-gray-900">Scan Jobs</span></nav> <h1 class="text-2xl font-bold text-gray-900">Scan Jobs</h1> <p class="mt-1 text-sm text-gray-500">Manage scheduled duplicate detection scans and view job history</p></div> <button class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors flex items-center gap-2"><svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg> Run Now</button></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="text-center py-12 text-gray-500">Loading scan jobs...</div>`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
