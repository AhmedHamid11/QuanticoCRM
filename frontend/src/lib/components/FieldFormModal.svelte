<script lang="ts">
	import EnumOptionsEditor from './EnumOptionsEditor.svelte';
	import RollupConfigEditor from './RollupConfigEditor.svelte';
	import type { EntityDef, FieldDef, FieldTypeInfo, FieldDefCreateInput, FieldType } from '$lib/types/admin';

	interface Props {
		mode: 'add' | 'edit';
		field: FieldDefCreateInput | FieldDef;
		fieldTypes: FieldTypeInfo[];
		allEntities: EntityDef[];
		enumOptions: string[];
		saving: boolean;
		onSave: () => void;
		onClose: () => void;
		onFieldChange: (field: FieldDefCreateInput | FieldDef) => void;
		onEnumOptionsChange: (options: string[]) => void;
		onLabelChange?: () => void;
		onApiNameChange?: () => void;
	}

	let {
		mode,
		field = $bindable(),
		fieldTypes,
		allEntities,
		enumOptions = $bindable(),
		saving,
		onSave,
		onClose,
		onFieldChange,
		onEnumOptionsChange,
		onLabelChange,
		onApiNameChange
	}: Props = $props();

	let fieldTypeKey = $derived(field.type || 'varchar');

	function getFieldTypeLabel(type: string): string {
		const typeInfo = fieldTypes.find(t => t.name === type);
		return typeInfo?.label || type;
	}

	function handleFieldUpdate(key: string, value: unknown) {
		const updated = { ...field, [key]: value };
		onFieldChange(updated as FieldDefCreateInput | FieldDef);
	}
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
	<div class="bg-white rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[90vh] overflow-y-auto">
		<div class="px-6 py-4 border-b border-gray-200">
			<h2 class="text-lg font-medium text-gray-900">
				{mode === 'add' ? 'Add New Field' : `Edit Field: ${field.label}`}
			</h2>
		</div>

		<div class="px-6 py-4 space-y-4">
			<!-- Label field -->
			<div>
				<label for="fieldLabel" class="block text-sm font-medium text-gray-700 mb-1">
					Label <span class="text-red-500">*</span>
				</label>
				<input
					id="fieldLabel"
					type="text"
					value={field.label}
					oninput={(e) => {
						handleFieldUpdate('label', e.currentTarget.value);
						if (mode === 'add' && onLabelChange) onLabelChange();
					}}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					placeholder="e.g., Company Name"
				/>
			</div>

			<!-- API Name field -->
			<div>
				<label for="fieldName" class="block text-sm font-medium text-gray-700 mb-1">
					API Name <span class="text-red-500">*</span>
				</label>
				<input
					id="fieldName"
					type="text"
					value={field.name}
					oninput={(e) => {
						if (mode === 'add') {
							handleFieldUpdate('name', e.currentTarget.value.replace(/\s+/g, '_').replace(/[^a-zA-Z0-9_]/g, ''));
							if (onApiNameChange) onApiNameChange();
						}
					}}
					disabled={mode === 'edit'}
					class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary font-mono text-sm {mode === 'edit' ? 'bg-gray-50 text-gray-500' : ''}"
					placeholder="e.g., companies_test"
				/>
				<p class="mt-1 text-xs text-gray-500">
					{mode === 'edit' ? 'API Name cannot be changed' : 'Internal field name (underscores instead of spaces, no special characters)'}
				</p>
			</div>

			<!-- Type field -->
			<div>
				<label for="fieldType" class="block text-sm font-medium text-gray-700 mb-1">
					Type <span class="text-red-500">*</span>
				</label>
				{#if mode === 'add'}
					<select
						id="fieldType"
						value={field.type}
						onchange={(e) => handleFieldUpdate('type', e.currentTarget.value as FieldType)}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					>
						{#each fieldTypes as type}
							<option value={type.name}>{type.label} - {type.description}</option>
						{/each}
					</select>
				{:else}
					<input
						type="text"
						value={getFieldTypeLabel(field.type)}
						disabled
						class="w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50 text-gray-500"
					/>
					<p class="mt-1 text-xs text-gray-500">Field type cannot be changed</p>
				{/if}
			</div>

			<!-- Enum options (for enum and multiEnum types) -->
			{#if fieldTypeKey === 'enum' || fieldTypeKey === 'multiEnum'}
				<EnumOptionsEditor
					options={enumOptions}
					onOptionsChange={onEnumOptionsChange}
					required={true}
				/>

				<!-- Default value selector for enum -->
				{#if fieldTypeKey === 'enum'}
					<div>
						<label for="defaultValue" class="block text-sm font-medium text-gray-700 mb-1">
							Default Value
						</label>
						<select
							id="defaultValue"
							value={(field as FieldDefCreateInput).defaultValue || ''}
							onchange={(e) => handleFieldUpdate('defaultValue' , e.currentTarget.value)}
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
						>
							<option value="">(No default)</option>
							{#each enumOptions.filter(o => o.trim()) as option}
								<option value={option}>{option}</option>
							{/each}
						</select>
						<p class="mt-1 text-xs text-gray-500">New records will have this value pre-selected</p>
					</div>
				{/if}
			{/if}

			<!-- Lookup entity selector (for link and linkMultiple types) -->
			{#if fieldTypeKey === 'link' || fieldTypeKey === 'linkMultiple'}
				<div>
					<label for="linkEntity" class="block text-sm font-medium text-gray-700 mb-1">
						Related Entity <span class="text-red-500">*</span>
					</label>
					<select
						id="linkEntity"
						value={(field as FieldDefCreateInput).linkEntity || ''}
						onchange={(e) => handleFieldUpdate('linkEntity' , e.currentTarget.value)}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					>
						<option value="">Select an entity...</option>
						{#each allEntities as ent}
							<option value={ent.name}>{ent.label}</option>
						{/each}
					</select>
					<p class="mt-1 text-xs text-gray-500">The entity this field will look up records from</p>
				</div>
			{/if}

			<!-- Rollup configuration -->
			{#if fieldTypeKey === 'rollup'}
				<RollupConfigEditor
					rollupResultType={(field as FieldDefCreateInput).rollupResultType}
					rollupQuery={(field as FieldDefCreateInput).rollupQuery}
					rollupDecimalPlaces={(field as FieldDefCreateInput).rollupDecimalPlaces}
					onResultTypeChange={(value) => handleFieldUpdate('rollupResultType' , value)}
					onQueryChange={(value) => handleFieldUpdate('rollupQuery' , value)}
					onDecimalPlacesChange={(value) => handleFieldUpdate('rollupDecimalPlaces' , value)}
				/>
			{/if}

			<!-- Text Block configuration -->
			{#if fieldTypeKey === 'textBlock'}
				<div>
					<label for="variant" class="block text-sm font-medium text-gray-700 mb-1">
						Style <span class="text-red-500">*</span>
					</label>
					<select
						id="variant"
						value={(field as FieldDefCreateInput).variant || 'info'}
						onchange={(e) => handleFieldUpdate('variant' , e.currentTarget.value)}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					>
						<option value="info">Info (Blue)</option>
						<option value="warning">Warning (Amber)</option>
						<option value="error">Error (Red)</option>
						<option value="success">Success (Green)</option>
					</select>
					<p class="mt-1 text-xs text-gray-500">Visual style of the message box</p>
				</div>

				<div>
					<label for="content" class="block text-sm font-medium text-gray-700 mb-1">
						Message Content <span class="text-red-500">*</span>
					</label>
					<textarea
						id="content"
						value={(field as FieldDefCreateInput).content || ''}
						oninput={(e) => handleFieldUpdate('content' , e.currentTarget.value)}
						rows={3}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
						placeholder="Enter your message text here..."
					></textarea>
					<p class="mt-1 text-xs text-gray-500">
						Use <code class="bg-gray-100 px-1 rounded">{'{{fieldName}}'}</code> to insert field values dynamically
					</p>
				</div>
			{/if}

			<!-- Max length for varchar (add mode only) -->
			{#if mode === 'add' && fieldTypeKey === 'varchar'}
				<div>
					<label for="maxLength" class="block text-sm font-medium text-gray-700 mb-1">
						Max Length
					</label>
					<input
						id="maxLength"
						type="number"
						value={(field as FieldDefCreateInput).maxLength ?? ''}
						oninput={(e) => handleFieldUpdate('maxLength' , parseInt(e.currentTarget.value) || undefined)}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
						placeholder="255"
					/>
				</div>
			{/if}

			<!-- Tooltip (edit mode) -->
			{#if mode === 'edit'}
				<div>
					<label for="editTooltip" class="block text-sm font-medium text-gray-700 mb-1">
						Tooltip / Help Text
					</label>
					<input
						id="editTooltip"
						type="text"
						value={(field as FieldDef).tooltip || ''}
						oninput={(e) => handleFieldUpdate('tooltip', e.currentTarget.value)}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
						placeholder="Help text shown on hover"
					/>
				</div>
			{/if}

			<!-- Checkboxes -->
			<div class="border-t border-gray-200 pt-4 space-y-3">
				<label class="flex items-center">
					<input
						type="checkbox"
						checked={field.isRequired}
						onchange={(e) => handleFieldUpdate('isRequired', e.currentTarget.checked)}
						class="rounded border-gray-300 text-primary focus:ring-primary"
					/>
					<span class="ml-2 text-sm text-gray-700">Required</span>
				</label>

				<label class="flex items-center">
					<input
						type="checkbox"
						checked={field.isReadOnly}
						onchange={(e) => handleFieldUpdate('isReadOnly', e.currentTarget.checked)}
						class="rounded border-gray-300 text-primary focus:ring-primary"
					/>
					<span class="ml-2 text-sm text-gray-700">Read Only</span>
				</label>

				<label class="flex items-center">
					<input
						type="checkbox"
						checked={field.isAudited}
						onchange={(e) => handleFieldUpdate('isAudited', e.currentTarget.checked)}
						class="rounded border-gray-300 text-primary focus:ring-primary"
					/>
					<span class="ml-2 text-sm text-gray-700">Audited (track changes)</span>
				</label>

				{#if fieldTypeKey === 'date' || fieldTypeKey === 'datetime'}
					<label class="flex items-center">
						<input
							type="checkbox"
							checked={(field as FieldDefCreateInput).defaultToToday ?? false}
							onchange={(e) => handleFieldUpdate('defaultToToday', e.currentTarget.checked)}
							class="rounded border-gray-300 text-primary focus:ring-primary"
						/>
						<span class="ml-2 text-sm text-gray-700">Default to Today</span>
					</label>
					<p class="ml-6 text-xs text-gray-500">New records will have this field set to the current date</p>
				{/if}
			</div>
		</div>

		<div class="px-6 py-4 border-t border-gray-200 flex justify-end gap-3">
			<button
				type="button"
				onclick={onClose}
				class="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={onSave}
				disabled={saving}
				class="px-4 py-2 bg-primary text-black rounded-md hover:bg-primary/90 disabled:opacity-50"
			>
				{saving ? (mode === 'add' ? 'Creating...' : 'Saving...') : (mode === 'add' ? 'Create Field' : 'Save Changes')}
			</button>
		</div>
	</div>
</div>
