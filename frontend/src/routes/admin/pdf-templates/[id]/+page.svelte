<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, post } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { DetailSkeleton, ErrorDisplay } from '$lib/components/ui';
	import type { PdfTemplate, PdfBranding, SectionConfig, AvailableField } from '$lib/types/pdf-template';
	import { BASE_DESIGNS, PAGE_SIZES, ORIENTATIONS, FONT_FAMILIES } from '$lib/types/pdf-template';

	let templateId = $derived($page.params.id);
	let template = $state<PdfTemplate | null>(null);
	let availableFields = $state<AvailableField[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Editable fields
	let name = $state('');
	let baseDesign = $state('professional');
	let pageSize = $state('A4');
	let orientation = $state('portrait');
	let margins = $state('20mm');
	let sections = $state<SectionConfig[]>([]);

	// Branding
	let logoUrl = $state('');
	let companyName = $state('');
	let primaryColor = $state('#1a56db');
	let accentColor = $state('#e5e7eb');
	let fontFamily = $state('Helvetica, Arial, sans-serif');

	// UI state
	let activePanel = $state<'branding' | 'sections' | 'page'>('sections');
	let expandedSection = $state<string | null>(null);
	let previewHtml = $state('');
	let loadingPreview = $state(false);
	let showAddFieldModal = $state(false);
	let addFieldSectionId = $state('');

	async function loadData() {
		try {
			loading = true;
			error = null;
			const [tpl, fields] = await Promise.all([
				get<PdfTemplate>(`/pdf-templates/${templateId}`),
				get<AvailableField[]>('/pdf-templates/available-fields?entityType=Quote').catch(() => [])
			]);

			template = tpl;
			availableFields = fields;

			name = tpl.name;
			baseDesign = tpl.baseDesign;
			pageSize = tpl.pageSize || 'A4';
			orientation = tpl.orientation || 'portrait';
			margins = tpl.margins || '20mm';
			sections = tpl.sections || [];

			if (tpl.branding) {
				logoUrl = tpl.branding.logoUrl || '';
				companyName = tpl.branding.companyName || '';
				primaryColor = tpl.branding.primaryColor || '#1a56db';
				accentColor = tpl.branding.accentColor || '#e5e7eb';
				fontFamily = tpl.branding.fontFamily || 'Helvetica, Arial, sans-serif';
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load template';
		} finally {
			loading = false;
		}
	}

	async function handleSave() {
		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}
		saving = true;
		try {
			const branding: PdfBranding = { logoUrl, companyName, primaryColor, accentColor, fontFamily };
			await put<PdfTemplate>(`/pdf-templates/${templateId}`, {
				name: name.trim(),
				baseDesign,
				pageSize,
				orientation,
				margins,
				branding,
				sections
			});
			toast.success('Template saved');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save template');
		} finally {
			saving = false;
		}
	}

	async function loadPreview() {
		loadingPreview = true;
		try {
			const result = await get<{ html: string }>(`/pdf-templates/${templateId}/preview`);
			previewHtml = result.html;
		} catch {
			previewHtml = '<div style="padding:40px;color:#999;text-align:center;">Preview unavailable. Create a quote first, then preview will use it as sample data.</div>';
		} finally {
			loadingPreview = false;
		}
	}

	function toggleSection(sectionId: string) {
		sections = sections.map(s =>
			s.id === sectionId ? { ...s, enabled: !s.enabled } : s
		);
	}

	function moveSection(from: number, to: number) {
		if (to < 0 || to >= sections.length) return;
		const arr = [...sections];
		const [item] = arr.splice(from, 1);
		arr.splice(to, 0, item);
		sections = arr;
	}

	function removeFieldFromSection(sectionId: string, fieldName: string) {
		sections = sections.map(s =>
			s.id === sectionId
				? { ...s, fields: s.fields.filter(f => f !== fieldName) }
				: s
		);
	}

	function moveFieldInSection(sectionId: string, from: number, to: number) {
		sections = sections.map(s => {
			if (s.id !== sectionId) return s;
			const fields = [...s.fields];
			if (to < 0 || to >= fields.length) return s;
			const [item] = fields.splice(from, 1);
			fields.splice(to, 0, item);
			return { ...s, fields };
		});
	}

	function addFieldToSection(sectionId: string, fieldName: string) {
		sections = sections.map(s =>
			s.id === sectionId && !s.fields.includes(fieldName)
				? { ...s, fields: [...s.fields, fieldName] }
				: s
		);
		showAddFieldModal = false;
	}

	function getFieldLabel(fieldName: string): string {
		const field = availableFields.find(f => f.name === fieldName);
		return field?.label ?? fieldName;
	}

	function getAvailableFieldsForSection(sectionId: string): AvailableField[] {
		const section = sections.find(s => s.id === sectionId);
		if (!section) return [];
		return availableFields.filter(
			f => f.section === sectionId && !section.fields.includes(f.name)
		);
	}

	onMount(() => loadData());
</script>

<div class="space-y-6">
	{#if loading}
		<DetailSkeleton />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else if template}
		<!-- Header -->
		<div class="flex items-center justify-between">
			<div>
				<div class="flex items-center gap-2">
					<a href="/admin/pdf-templates" class="text-gray-400 hover:text-gray-600">
						<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
					</a>
					<h1 class="text-2xl font-bold text-gray-900">Edit Template</h1>
				</div>
				<p class="mt-1 text-sm text-gray-500 ml-7">{template.name}</p>
			</div>
			<div class="flex items-center gap-3">
				<button onclick={loadPreview}
					disabled={loadingPreview}
					class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50">
					{loadingPreview ? 'Loading...' : 'Preview'}
				</button>
				<button onclick={handleSave}
					disabled={saving}
					class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-600/90 disabled:opacity-50">
					{saving ? 'Saving...' : 'Save Template'}
				</button>
			</div>
		</div>

		<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
			<!-- Left: Config Panels -->
			<div class="lg:col-span-2 space-y-4">
				<!-- Template Name & Base Design -->
				<div class="bg-white shadow rounded-lg p-6 space-y-4">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="tplName" class="block text-sm font-medium text-gray-700 mb-1">Template Name</label>
							<input id="tplName" type="text" bind:value={name}
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
						</div>
						<div>
							<label for="tplDesign" class="block text-sm font-medium text-gray-700 mb-1">Base Design</label>
							<select id="tplDesign" bind:value={baseDesign}
								class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
								{#each BASE_DESIGNS as d}
									<option value={d.value}>{d.label}</option>
								{/each}
							</select>
						</div>
					</div>
				</div>

				<!-- Panel Tabs -->
				<div class="crm-card overflow-hidden">
					<div class="border-b border-gray-200">
						<nav class="flex -mb-px">
							<button
								onclick={() => activePanel = 'sections'}
								class="px-6 py-3 text-sm font-medium border-b-2 {activePanel === 'sections' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">
								Sections & Fields
							</button>
							<button
								onclick={() => activePanel = 'branding'}
								class="px-6 py-3 text-sm font-medium border-b-2 {activePanel === 'branding' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">
								Branding
							</button>
							<button
								onclick={() => activePanel = 'page'}
								class="px-6 py-3 text-sm font-medium border-b-2 {activePanel === 'page' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}">
								Page Settings
							</button>
						</nav>
					</div>

					<div class="p-6">
						{#if activePanel === 'sections'}
							<!-- Sections Panel -->
							<div class="space-y-3">
								<p class="text-sm text-gray-500 mb-4">
									Toggle sections on/off, reorder them, and manage which fields appear in each section.
								</p>
								{#each sections as section, i (section.id)}
									<div class="border rounded-lg {section.enabled ? 'border-gray-200' : 'border-gray-100 opacity-60'}">
										<div class="flex items-center gap-3 px-4 py-3">
											<!-- Reorder buttons -->
											<div class="flex flex-col gap-0.5">
												<button type="button" onclick={() => moveSection(i, i - 1)}
													disabled={i === 0}
													class="p-0.5 text-gray-400 hover:text-gray-600 disabled:opacity-30"
													title="Move up">
													<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
														<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
													</svg>
												</button>
												<button type="button" onclick={() => moveSection(i, i + 1)}
													disabled={i === sections.length - 1}
													class="p-0.5 text-gray-400 hover:text-gray-600 disabled:opacity-30"
													title="Move down">
													<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
														<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
													</svg>
												</button>
											</div>

											<!-- Toggle -->
											<button type="button" onclick={() => toggleSection(section.id)}
												class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors {section.enabled ? 'bg-blue-600' : 'bg-gray-200'}"
												role="switch" aria-checked={section.enabled}>
												<span class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition-transform {section.enabled ? 'translate-x-4' : 'translate-x-0'}"></span>
											</button>

											<!-- Label -->
											<span class="flex-1 text-sm font-medium text-gray-900">{section.label}</span>
											<span class="text-xs text-gray-400">{section.fields.length} fields</span>

											<!-- Expand -->
											<button type="button" onclick={() => expandedSection = expandedSection === section.id ? null : section.id}
												class="p-1 text-gray-400 hover:text-gray-600">
												<svg class="w-4 h-4 transition-transform {expandedSection === section.id ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
												</svg>
											</button>
										</div>

										{#if expandedSection === section.id}
											<div class="px-4 pb-4 border-t border-gray-100 pt-3">
												{#if section.fields.length === 0}
													<p class="text-sm text-gray-400 italic">No fields configured</p>
												{:else}
													<div class="space-y-1">
														{#each section.fields as field, fi (field)}
															<div class="flex items-center gap-2 py-1 px-2 rounded hover:bg-gray-50 group">
																<div class="flex flex-col gap-0.5">
																	<button type="button" onclick={() => moveFieldInSection(section.id, fi, fi - 1)}
																		disabled={fi === 0}
																		class="p-0.5 text-gray-300 hover:text-gray-500 disabled:opacity-30">
																		<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
																			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
																		</svg>
																	</button>
																	<button type="button" onclick={() => moveFieldInSection(section.id, fi, fi + 1)}
																		disabled={fi === section.fields.length - 1}
																		class="p-0.5 text-gray-300 hover:text-gray-500 disabled:opacity-30">
																		<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
																			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
																		</svg>
																	</button>
																</div>
																<span class="flex-1 text-sm text-gray-700">{getFieldLabel(field)}</span>
																<button type="button" onclick={() => removeFieldFromSection(section.id, field)}
																	class="p-1 text-gray-300 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity"
																	title="Remove field">
																	<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
																		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
																	</svg>
																</button>
															</div>
														{/each}
													</div>
												{/if}

												{#if getAvailableFieldsForSection(section.id).length > 0}
													<button type="button"
														onclick={() => { addFieldSectionId = section.id; showAddFieldModal = true; }}
														class="mt-2 inline-flex items-center px-2 py-1 text-xs font-medium text-blue-700 bg-blue-50 rounded hover:bg-blue-100">
														<svg class="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
															<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
														</svg>
														Add Field
													</button>
												{/if}
											</div>
										{/if}
									</div>
								{/each}
							</div>

						{:else if activePanel === 'branding'}
							<!-- Branding Panel -->
							<div class="space-y-4">
								<div>
									<label for="logoUrl" class="block text-sm font-medium text-gray-700 mb-1">Logo URL</label>
									<input id="logoUrl" type="url" bind:value={logoUrl} placeholder="https://example.com/logo.png"
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
									{#if logoUrl}
										<div class="mt-2 p-2 border rounded inline-block">
											<img src={logoUrl} alt="Logo preview" class="h-10 max-w-[200px] object-contain"
												onerror={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
										</div>
									{/if}
								</div>
								<div>
									<label for="companyName" class="block text-sm font-medium text-gray-700 mb-1">Company Name</label>
									<input id="companyName" type="text" bind:value={companyName} placeholder="Your Company Inc."
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
								</div>
								<div class="grid grid-cols-2 gap-4">
									<div>
										<label for="primaryColor" class="block text-sm font-medium text-gray-700 mb-1">Primary Color</label>
										<div class="flex items-center gap-2">
											<input id="primaryColor" type="color" bind:value={primaryColor}
												class="w-10 h-10 border border-gray-300 rounded cursor-pointer" />
											<input type="text" bind:value={primaryColor}
												class="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm font-mono" />
										</div>
									</div>
									<div>
										<label for="accentColor" class="block text-sm font-medium text-gray-700 mb-1">Accent Color</label>
										<div class="flex items-center gap-2">
											<input id="accentColor" type="color" bind:value={accentColor}
												class="w-10 h-10 border border-gray-300 rounded cursor-pointer" />
											<input type="text" bind:value={accentColor}
												class="flex-1 px-3 py-2 border border-gray-300 rounded-md text-sm font-mono" />
										</div>
									</div>
								</div>
								<div>
									<label for="fontFamily" class="block text-sm font-medium text-gray-700 mb-1">Font Family</label>
									<select id="fontFamily" bind:value={fontFamily}
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
										{#each FONT_FAMILIES as ff}
											<option value={ff}>{ff}</option>
										{/each}
									</select>
									<p class="mt-1 text-xs text-gray-400" style="font-family: {fontFamily}">
										The quick brown fox jumps over the lazy dog.
									</p>
								</div>
							</div>

						{:else if activePanel === 'page'}
							<!-- Page Settings Panel -->
							<div class="space-y-4">
								<div class="grid grid-cols-2 gap-4">
									<div>
										<label for="pageSize" class="block text-sm font-medium text-gray-700 mb-1">Page Size</label>
										<select id="pageSize" bind:value={pageSize}
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
											{#each PAGE_SIZES as ps}
												<option value={ps}>{ps}</option>
											{/each}
										</select>
									</div>
									<div>
										<label for="orientation" class="block text-sm font-medium text-gray-700 mb-1">Orientation</label>
										<select id="orientation" bind:value={orientation}
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500">
											{#each ORIENTATIONS as o}
												<option value={o}>{o.charAt(0).toUpperCase() + o.slice(1)}</option>
											{/each}
										</select>
									</div>
								</div>
								<div>
									<label for="margins" class="block text-sm font-medium text-gray-700 mb-1">Margins</label>
									<input id="margins" type="text" bind:value={margins} placeholder="20mm"
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500" />
									<p class="mt-1 text-xs text-gray-400">Use CSS units (e.g. 20mm, 0.75in, 2cm)</p>
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>

			<!-- Right: Preview Panel -->
			<div class="lg:col-span-1">
				<div class="bg-white shadow rounded-lg overflow-hidden sticky top-4">
					<div class="px-4 py-3 bg-gray-50 border-b border-gray-200 flex items-center justify-between">
						<h3 class="text-sm font-medium text-gray-700">Preview</h3>
						<button onclick={loadPreview}
							disabled={loadingPreview}
							class="text-xs text-blue-600 hover:text-blue-800 disabled:opacity-50">
							{loadingPreview ? 'Loading...' : 'Refresh'}
						</button>
					</div>
					<div class="p-2">
						{#if previewHtml}
							<div class="border border-gray-200 rounded bg-white overflow-auto" style="max-height: 70vh;">
								<iframe
									srcdoc={previewHtml}
									title="PDF Preview"
									class="w-full border-0"
									style="min-height: 500px; transform: scale(0.6); transform-origin: top left; width: 166.67%;"
									sandbox="allow-same-origin"
								></iframe>
							</div>
						{:else}
							<div class="flex items-center justify-center py-20 text-gray-400">
								<div class="text-center">
									<svg class="mx-auto h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
									</svg>
									<p class="mt-2 text-sm">Click "Preview" to see the template</p>
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>

<!-- Add Field Modal -->
{#if showAddFieldModal}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 bg-gray-500/75 flex items-center justify-center z-50"
		onclick={(e) => { if (e.target === e.currentTarget) showAddFieldModal = false; }}>
		<div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
			<div class="px-6 py-4 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">Add Field</h2>
				<p class="text-sm text-gray-500 mt-1">
					Select a field to add to the "{sections.find(s => s.id === addFieldSectionId)?.label}" section.
				</p>
			</div>
			<div class="p-4 max-h-64 overflow-y-auto">
				{#each getAvailableFieldsForSection(addFieldSectionId) as field}
					<button onclick={() => addFieldToSection(addFieldSectionId, field.name)}
						class="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 rounded">
						{field.label}
					</button>
				{/each}
			</div>
			<div class="px-6 py-3 border-t border-gray-200">
				<button onclick={() => showAddFieldModal = false}
					class="w-full px-4 py-2 text-sm text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">
					Cancel
				</button>
			</div>
		</div>
	</div>
{/if}
