<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import type { LayoutSectionV2 } from '$lib/types/layout';
	import type { FieldValidationError } from '$lib/types/validation';
	import { evaluateVisibility } from '$lib/types/layout';
	import { fieldNameToKey } from '$lib/utils/fieldMapping';
	import LookupField from './LookupField.svelte';
	import MultiLookupField from './MultiLookupField.svelte';
	import StreamField from './StreamField.svelte';

	interface LookupRecord {
		id: string;
		name: string;
	}

	interface Props {
		section: LayoutSectionV2;
		fields: FieldDef[];
		formData: Record<string, unknown>;
		lookupNames: Record<string, string>;
		multiLookupValues: Record<string, LookupRecord[]>;
		getFieldError: (fieldName: string) => FieldValidationError | undefined;
		onLookupChange: (fieldName: string, id: string | null, name: string) => void;
		onMultiLookupChange: (fieldName: string, values: LookupRecord[]) => void;
	}

	let {
		section,
		fields,
		formData = $bindable(),
		lookupNames,
		multiLookupValues,
		getFieldError,
		onLookupChange,
		onMultiLookupChange
	}: Props = $props();

	// Get visible fields within the section
	let visibleFields = $derived(
		section.fields.filter((f) => evaluateVisibility(f.visibility, formData))
	);

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find((f) => f.name === fieldName);
	}

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
</script>

{#if visibleFields.length > 0}
	<div class="bg-white shadow rounded-lg overflow-hidden">
		<!-- Section Header -->
		<div class="px-6 py-4 bg-gray-50 border-b border-gray-200">
			<h2 class="text-lg font-medium text-gray-900">{section.label}</h2>
		</div>

		<!-- Section Content -->
		<div class="p-6">
			<div
				class="grid gap-x-8 gap-y-4"
				style="grid-template-columns: repeat({section.columns}, minmax(0, 1fr))"
			>
				{#each visibleFields as fieldLayout (fieldLayout.name)}
					{@const field = getFieldDef(fieldLayout.name)}
					{#if field && !field.isReadOnly && field.type !== 'rollup' && field.name !== 'id'}
						{@const fieldError = getFieldError(field.name)}
						{@const isWideField = field.type === 'text' || field.type === 'stream'}
						<div class={isWideField ? 'col-span-full' : ''}>
							{#if field.type === 'link' && field.linkEntity}
								<LookupField
									entity={field.linkEntity}
									value={formData[`${field.name}Id`] as string | null}
									valueName={lookupNames[field.name] || ''}
									label={field.label}
									required={field.isRequired}
									onchange={(id, name) => {
										onLookupChange(field.name, id, name);
									}}
								/>
							{:else if field.type === 'linkMultiple' && field.linkEntity}
								<MultiLookupField
									entity={field.linkEntity}
									values={multiLookupValues[field.name] || []}
									label={field.label}
									required={field.isRequired}
									onchange={(values) => {
										onMultiLookupChange(field.name, values);
									}}
								/>
							{:else if field.type === 'stream'}
								<StreamField
									label={field.label}
									bind:entry={formData[field.name]}
									log={formData[`${field.name}Log`] ? String(formData[`${field.name}Log`]) : ''}
									readonly={false}
								/>
							{:else}
								<label
									for={field.name}
									class="block text-sm font-medium mb-1"
									class:text-gray-700={!fieldError}
									class:text-red-700={fieldError}
								>
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
										class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										class:border-gray-300={!fieldError}
										class:border-red-500={fieldError}
									></textarea>
								{:else if field.type === 'bool'}
									<input
										type="checkbox"
										id={field.name}
										bind:checked={formData[field.name]}
										class="w-4 h-4 rounded text-blue-600 focus:ring-blue-500"
										class:border-gray-300={!fieldError}
										class:border-red-500={fieldError}
									/>
								{:else if field.type === 'enum' && field.options}
									<select
										id={field.name}
										bind:value={formData[field.name]}
										required={field.isRequired}
										class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										class:border-gray-300={!fieldError}
										class:border-red-500={fieldError}
									>
										<option value="">-- Select --</option>
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
										class="w-full px-3 py-2 border rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										class:border-gray-300={!fieldError}
										class:border-red-500={fieldError}
									/>
								{/if}

								{#if fieldError}
									<p class="mt-1 text-xs text-red-600">{fieldError.message}</p>
								{:else if field.tooltip}
									<p class="mt-1 text-xs text-gray-500">{field.tooltip}</p>
								{/if}
							{/if}
						</div>
					{/if}
				{/each}
			</div>
		</div>
	</div>
{/if}
