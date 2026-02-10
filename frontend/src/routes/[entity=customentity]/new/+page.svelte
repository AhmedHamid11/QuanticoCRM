<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, post } from '$lib/utils/api';
	import { getEntityNameFromPath } from '$lib/stores/navigation.svelte';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import LookupField from '$lib/components/LookupField.svelte';
	import MultiLookupField from '$lib/components/MultiLookupField.svelte';
	import StreamField from '$lib/components/StreamField.svelte';

	interface LookupRecord {
		id: string;
		name: string;
	}

	let entitySlug = $derived($page.params.entity!);
	let entityName = $derived(getEntityNameFromPath(entitySlug) || toPascalCase(entitySlug));
	let queryParams = $derived($page.url.searchParams);

	function toPascalCase(slug: string): string {
		let singular = slug;
		if (slug.endsWith('s') && slug.length > 1) {
			singular = slug.slice(0, -1);
		}
		return singular.charAt(0).toUpperCase() + singular.slice(1);
	}

	let entityDef = $state<EntityDef | null>(null);
	let fields = $state<FieldDef[]>([]);
	let formData = $state<Record<string, unknown>>({});
	let lookupNames = $state<Record<string, string>>({});
	let multiLookupValues = $state<Record<string, LookupRecord[]>>({});
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Editable fields (exclude id, system fields)
	let editableFields = $derived(
		fields.filter(f => f.name !== 'id' && !f.isReadOnly)
	);

	// Parse enum options (handles both JSON array and comma-separated formats)
	function getEnumOptions(field: FieldDef): string[] {
		if (!field.options) return [];
		const opts = field.options.trim();
		if (opts.startsWith('[')) {
			try {
				return JSON.parse(opts);
			} catch {
				return [];
			}
		}
		return opts.split(',').map(o => o.trim());
	}

	async function loadEntityDef() {
		try {
			// Use public endpoint (doesn't require admin role)
			entityDef = await get<EntityDef>(`/entities/${entityName}/def`);
		} catch {
			entityDef = null;
		}
	}

	async function loadFields() {
		try {
			// Use public endpoint (doesn't require admin role)
			fields = await get<FieldDef[]>(`/entities/${entityName}/fields`);
			// Initialize form data with default values
			for (const field of fields) {
				if (field.name !== 'id' && !field.isReadOnly) {
					// Check if date/datetime field should default to today
					if ((field.type === 'date' || field.type === 'datetime') && field.defaultToToday) {
						const today = new Date();
						if (field.type === 'date') {
							formData[field.name] = today.toISOString().split('T')[0]; // YYYY-MM-DD
						} else {
							// datetime-local format: YYYY-MM-DDTHH:MM
							formData[field.name] = today.toISOString().slice(0, 16);
						}
					} else {
						formData[field.name] = field.defaultValue || '';
					}
				}
			}

			// Pre-fill lookup fields from URL query parameters
			await prefillFromQueryParams();
		} catch {
			fields = [];
		}
	}

	// Pre-fill lookup fields from URL query parameters (e.g., ?jobId=xyz)
	async function prefillFromQueryParams() {
		for (const field of fields) {
			if (field.type === 'link' && field.linkEntity) {
				// Check for both fieldNameId and fieldName patterns
				const idParamKey = `${field.name}Id`;
				const simpleParamKey = field.name;
				const prefillId = queryParams.get(idParamKey) || queryParams.get(simpleParamKey);

				if (prefillId) {
					// Set the ID in form data
					formData[`${field.name}Id`] = prefillId;

					// Fetch the record to get the display name
					try {
						const record = await get<Record<string, unknown>>(
							`/entities/${field.linkEntity}/records/${prefillId}`
						);
						const displayName = (record.name as string) || prefillId;
						formData[`${field.name}Name`] = displayName;
						lookupNames[field.name] = displayName;
					} catch {
						// If fetch fails, just use the ID as the name
						formData[`${field.name}Name`] = prefillId;
						lookupNames[field.name] = prefillId;
					}
				}
			}
		}
	}

	async function handleSubmit() {
		saving = true;
		error = null;

		try {
			const created = await post<Record<string, unknown>>(`/entities/${entityName}/records`, formData);
			goto(`/${entitySlug}/${created.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create record';
		} finally {
			saving = false;
		}
	}

	function getInputType(field: FieldDef): string {
		switch (field.type) {
			case 'email': return 'email';
			case 'url': return 'url';
			case 'phone': return 'tel';
			case 'int': case 'float': case 'currency': return 'number';
			case 'date': return 'date';
			case 'datetime': return 'datetime-local';
			default: return 'text';
		}
	}

	// Reload data when entity changes (handles navigation between new pages of different entities)
	$effect(() => {
		// Track these reactive values to trigger reload on navigation
		const _entity = entityName;
		const _queryParams = queryParams.toString();

		// Reset state
		entityDef = null;
		fields = [];
		formData = {};
		lookupNames = {};
		multiLookupValues = {};
		loading = true;
		error = null;

		// Load data
		(async () => {
			await Promise.all([loadEntityDef(), loadFields()]);
			loading = false;
		})();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<div>
		<nav class="text-sm text-gray-500 mb-2">
			<a href="/{entitySlug}" class="hover:text-gray-700">{entityDef?.labelPlural || entityName + 's'}</a>
			<span class="mx-2">/</span>
			<span class="text-gray-900">New</span>
		</nav>
		<h1 class="text-2xl font-bold text-gray-900">
			New {entityDef?.label || entityName}
		</h1>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gray-500">Loading...</div>
	{:else}
		<div class="bg-white shadow rounded-lg p-6 max-w-2xl">
			{#if error}
				<div class="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
					{error}
				</div>
			{/if}

			<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
				<div class="space-y-4">
					{#each editableFields as field}
						<div>
							{#if field.type === 'link' && field.linkEntity}
								<LookupField
									entity={field.linkEntity}
									value={formData[`${field.name}Id`] as string | null}
									valueName={lookupNames[field.name] || ''}
									label={field.label}
									required={field.isRequired}
									onchange={(id, name) => {
										// Store both ID and Name for lookup fields per standard
										formData[`${field.name}Id`] = id;
										formData[`${field.name}Name`] = name;
										lookupNames[field.name] = name;
									}}
								/>
							{:else if field.type === 'linkMultiple' && field.linkEntity}
								<MultiLookupField
									entity={field.linkEntity}
									values={multiLookupValues[field.name] || []}
									label={field.label}
									required={field.isRequired}
									onchange={(values) => {
										multiLookupValues[field.name] = values;
										// Store as JSON arrays for the API
										formData[`${field.name}Ids`] = JSON.stringify(values.map(v => v.id));
										formData[`${field.name}Names`] = JSON.stringify(values.map(v => v.name));
									}}
								/>
							{:else if field.type === 'stream'}
								<StreamField
									label={field.label}
									entry={String(formData[field.name] || '')}
									log=""
									required={field.isRequired}
									onchange={(value) => {
										formData[field.name] = value;
									}}
								/>
							{:else}
							<label for={field.name} class="block text-sm font-medium text-gray-700 mb-1">
								{field.label}
								{#if field.isRequired}
									<span class="text-red-500">*</span>
								{/if}
							</label>

							{#if field.type === 'text'}
								<textarea
									id={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									rows="3"
									class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
								></textarea>
							{:else if field.type === 'bool'}
								<input
									type="checkbox"
									id={field.name}
									checked={!!formData[field.name]}
									onchange={(e) => { formData[field.name] = e.currentTarget.checked; }}
									class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
								/>
							{:else if field.type === 'enum' && field.options}
								<select
									id={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
								>
									<option value="">Select...</option>
									{#each getEnumOptions(field) as option}
										<option value={option}>{option}</option>
									{/each}
								</select>
							{:else if field.type === 'multiEnum' && field.options}
								{@const options = getEnumOptions(field)}
								{@const selectedValues = (() => {
									const val = formData[field.name];
									if (!val) return [];
									if (typeof val === 'string' && val.startsWith('[')) {
										try { return JSON.parse(val); } catch { return []; }
									}
									return Array.isArray(val) ? val : [];
								})()}
								<div class="space-y-2">
									{#each options as option}
										<label class="flex items-center gap-2">
											<input
												type="checkbox"
												checked={selectedValues.includes(option)}
												onchange={(e) => {
													const checked = (e.target as HTMLInputElement).checked;
													let current = [...selectedValues];
													if (checked && !current.includes(option)) {
														current.push(option);
													} else if (!checked) {
														current = current.filter(v => v !== option);
													}
													formData[field.name] = JSON.stringify(current);
												}}
												class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
											/>
											<span class="text-sm text-gray-700">{option}</span>
										</label>
									{/each}
								</div>
							{:else}
								<input
									type={getInputType(field)}
									id={field.name}
									bind:value={formData[field.name]}
									required={field.isRequired}
									maxlength={field.maxLength || undefined}
									min={field.minValue || undefined}
									max={field.maxValue || undefined}
									step={field.type === 'float' || field.type === 'currency' ? '0.01' : undefined}
									class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
								/>
							{/if}

							{#if field.tooltip}
								<p class="mt-1 text-xs text-gray-500">{field.tooltip}</p>
							{/if}
							{/if}
						</div>
					{/each}
				</div>

				<div class="mt-6 flex justify-end gap-3">
					<a
						href="/{entitySlug}"
						class="px-4 py-2 text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
					>
						Cancel
					</a>
					<button
						type="submit"
						disabled={saving}
						class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-600/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{saving ? 'Creating...' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	{/if}
</div>
