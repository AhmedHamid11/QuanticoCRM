<script lang="ts">
	import type { FlowStep, ScreenField, DecisionOutcome, AssignmentOperation, FieldOption } from '$lib/types/flow';
	import { FIELD_TYPES } from '$lib/types/flow';
	import type { FieldDef } from '$lib/types/admin';

	interface Props {
		step: FlowStep;
		allSteps: FlowStep[];
		fields: FieldDef[];
		entities: { name: string; label: string }[];
		onUpdate: (step: FlowStep) => void;
		onDelete: () => void;
	}

	let { step, allSteps, fields, entities, onUpdate, onDelete }: Props = $props();

	function updateStep<K extends keyof FlowStep>(key: K, value: FlowStep[K]) {
		onUpdate({ ...step, [key]: value } as FlowStep);
	}

	// Screen step helpers
	function addField() {
		if (step.type !== 'screen') return;
		const newField: ScreenField = {
			name: `field_${Date.now()}`,
			label: 'New Field',
			type: 'text',
			required: false
		};
		onUpdate({ ...step, fields: [...step.fields, newField] });
	}

	function updateField(index: number, updates: Partial<ScreenField>) {
		if (step.type !== 'screen') return;
		const newFields = [...step.fields];
		newFields[index] = { ...newFields[index], ...updates };
		onUpdate({ ...step, fields: newFields });
	}

	function removeField(index: number) {
		if (step.type !== 'screen') return;
		onUpdate({ ...step, fields: step.fields.filter((_, i) => i !== index) });
	}

	function addFieldOption(fieldIndex: number) {
		if (step.type !== 'screen') return;
		const field = step.fields[fieldIndex];
		const newOption: FieldOption = { value: '', label: '' };
		updateField(fieldIndex, { options: [...(field.options || []), newOption] });
	}

	function updateFieldOption(fieldIndex: number, optionIndex: number, updates: Partial<FieldOption>) {
		if (step.type !== 'screen') return;
		const field = step.fields[fieldIndex];
		const newOptions = [...(field.options || [])];
		newOptions[optionIndex] = { ...newOptions[optionIndex], ...updates };
		updateField(fieldIndex, { options: newOptions });
	}

	function removeFieldOption(fieldIndex: number, optionIndex: number) {
		if (step.type !== 'screen') return;
		const field = step.fields[fieldIndex];
		updateField(fieldIndex, { options: field.options?.filter((_, i) => i !== optionIndex) });
	}

	// Decision step helpers
	function addOutcome() {
		if (step.type !== 'decision') return;
		const newOutcome: DecisionOutcome = {
			id: `outcome_${Date.now()}`,
			label: 'New Outcome',
			condition: ''
		};
		onUpdate({ ...step, outcomes: [...step.outcomes, newOutcome] });
	}

	function updateOutcome(index: number, updates: Partial<DecisionOutcome>) {
		if (step.type !== 'decision') return;
		const newOutcomes = [...step.outcomes];
		newOutcomes[index] = { ...newOutcomes[index], ...updates };
		onUpdate({ ...step, outcomes: newOutcomes });
	}

	function removeOutcome(index: number) {
		if (step.type !== 'decision') return;
		onUpdate({ ...step, outcomes: step.outcomes.filter((_, i) => i !== index) });
	}

	// Assignment step helpers
	function addAssignment() {
		if (step.type !== 'assignment') return;
		const newAssignment: AssignmentOperation = {
			variable: '',
			operation: 'set',
			value: ''
		};
		onUpdate({ ...step, assignments: [...step.assignments, newAssignment] });
	}

	function updateAssignment(index: number, updates: Partial<AssignmentOperation>) {
		if (step.type !== 'assignment') return;
		const newAssignments = [...step.assignments];
		newAssignments[index] = { ...newAssignments[index], ...updates };
		onUpdate({ ...step, assignments: newAssignments });
	}

	function removeAssignment(index: number) {
		if (step.type !== 'assignment') return;
		onUpdate({ ...step, assignments: step.assignments.filter((_, i) => i !== index) });
	}

	// Field mapping helpers for record steps
	function updateFieldMapping(field: string, value: string) {
		if (step.type !== 'record_create' && step.type !== 'record_update') return;
		const newMappings = { ...step.fieldMappings, [field]: value };
		if (!value) delete newMappings[field];
		onUpdate({ ...step, fieldMappings: newMappings });
	}

	// Get available step IDs for next selector
	function getNextStepOptions() {
		return allSteps.filter(s => s.id !== step.id);
	}

	function needsOptions(type: string) {
		return ['select', 'radio'].includes(type);
	}
</script>

<div class="border border-gray-200 rounded-lg p-4 bg-white shadow-sm">
	<!-- Header with delete -->
	<div class="flex justify-between items-start mb-4">
		<div class="flex-1">
			<input
				type="text"
				value={step.name}
				onchange={(e) => updateStep('name', e.currentTarget.value)}
				class="text-lg font-medium text-gray-900 border-0 border-b border-transparent hover:border-gray-300 focus:border-primary focus:ring-0 p-0 w-full bg-transparent"
				placeholder="Step name"
			/>
			<span class="text-xs text-gray-400 mt-1 block">ID: {step.id}</span>
		</div>
		<button
			type="button"
			onclick={onDelete}
			class="text-red-600 hover:text-red-800 text-sm ml-4"
		>
			Delete
		</button>
	</div>

	<!-- Step type specific content -->
	{#if step.type === 'screen'}
		<!-- Screen Header -->
		<div class="mb-4">
			<label class="block text-sm font-medium text-gray-700 mb-1">Header Alert (optional)</label>
			<div class="grid grid-cols-2 gap-2">
				<select
					value={step.header?.variant || ''}
					onchange={(e) => {
						const variant = e.currentTarget.value as 'success' | 'warning' | 'error' | 'info';
						if (variant) {
							onUpdate({ ...step, header: { type: 'alert', variant, message: step.header?.message || '' } });
						} else {
							const { header, ...rest } = step;
							onUpdate({ ...rest, type: 'screen', fields: step.fields });
						}
					}}
					class="rounded-md border-gray-300 text-sm"
				>
					<option value="">None</option>
					<option value="info">Info</option>
					<option value="success">Success</option>
					<option value="warning">Warning</option>
					<option value="error">Error</option>
				</select>
				{#if step.header}
					<input
						type="text"
						value={step.header.message}
						onchange={(e) => onUpdate({ ...step, header: { ...step.header!, message: e.currentTarget.value } })}
						placeholder="Alert message"
						class="rounded-md border-gray-300 text-sm"
					/>
				{/if}
			</div>
		</div>

		<!-- Fields -->
		<div class="space-y-3">
			<div class="flex justify-between items-center">
				<label class="text-sm font-medium text-gray-700">Fields</label>
				<button type="button" onclick={addField} class="text-sm text-primary hover:text-blue-800">
					+ Add Field
				</button>
			</div>

			{#each step.fields as field, index (field.name + index)}
				<div class="border border-gray-200 rounded p-3 bg-gray-50 space-y-2">
					<div class="flex justify-between items-start">
						<div class="grid grid-cols-3 gap-2 flex-1">
							<input
								type="text"
								value={field.name}
								onchange={(e) => updateField(index, { name: e.currentTarget.value })}
								placeholder="Field name"
								class="text-sm rounded-md border-gray-300"
							/>
							<input
								type="text"
								value={field.label}
								onchange={(e) => updateField(index, { label: e.currentTarget.value })}
								placeholder="Label"
								class="text-sm rounded-md border-gray-300"
							/>
							<select
								value={field.type}
								onchange={(e) => updateField(index, { type: e.currentTarget.value as ScreenField['type'] })}
								class="text-sm rounded-md border-gray-300"
							>
								{#each FIELD_TYPES as ft}
									<option value={ft.value}>{ft.label}</option>
								{/each}
							</select>
						</div>
						<button
							type="button"
							onclick={() => removeField(index)}
							class="text-red-600 hover:text-red-800 text-sm ml-2"
						>
							&times;
						</button>
					</div>

					<div class="grid grid-cols-3 gap-2">
						<label class="flex items-center text-sm text-gray-600">
							<input
								type="checkbox"
								checked={field.required || false}
								onchange={(e) => updateField(index, { required: e.currentTarget.checked })}
								class="mr-2 rounded"
							/>
							Required
						</label>
						<input
							type="text"
							value={field.defaultValue || ''}
							onchange={(e) => updateField(index, { defaultValue: e.currentTarget.value || undefined })}
							placeholder="Default value"
							class="text-sm rounded-md border-gray-300"
						/>
						<input
							type="text"
							value={field.placeholder || ''}
							onchange={(e) => updateField(index, { placeholder: e.currentTarget.value || undefined })}
							placeholder="Placeholder"
							class="text-sm rounded-md border-gray-300"
						/>
					</div>

					<input
						type="text"
						value={field.helpText || ''}
						onchange={(e) => updateField(index, { helpText: e.currentTarget.value || undefined })}
						placeholder="Help text"
						class="w-full text-sm rounded-md border-gray-300"
					/>

					<!-- Options for select/radio -->
					{#if needsOptions(field.type)}
						<div class="mt-2 pl-4 border-l-2 border-gray-200">
							<div class="flex justify-between items-center mb-1">
								<span class="text-xs font-medium text-gray-500">Options</span>
								<button type="button" onclick={() => addFieldOption(index)} class="text-xs text-primary">+ Add</button>
							</div>
							{#each field.options || [] as opt, optIndex}
								<div class="flex gap-2 mb-1">
									<input
										type="text"
										value={opt.value}
										onchange={(e) => updateFieldOption(index, optIndex, { value: e.currentTarget.value })}
										placeholder="Value"
										class="text-xs rounded border-gray-300 flex-1"
									/>
									<input
										type="text"
										value={opt.label}
										onchange={(e) => updateFieldOption(index, optIndex, { label: e.currentTarget.value })}
										placeholder="Label"
										class="text-xs rounded border-gray-300 flex-1"
									/>
									<button type="button" onclick={() => removeFieldOption(index, optIndex)} class="text-red-600 text-xs">&times;</button>
								</div>
							{/each}
						</div>
					{/if}

					<!-- Lookup entity -->
					{#if field.type === 'lookup'}
						<select
							value={field.entity || ''}
							onchange={(e) => updateField(index, { entity: e.currentTarget.value || undefined })}
							class="text-sm rounded-md border-gray-300 w-full"
						>
							<option value="">Select entity</option>
							{#each entities as entity}
								<option value={entity.name}>{entity.label}</option>
							{/each}
						</select>
					{/if}
				</div>
			{/each}

			{#if step.fields.length === 0}
				<p class="text-sm text-gray-500 text-center py-4">No fields. Add a field to collect user input.</p>
			{/if}
		</div>

		<!-- Next step -->
		<div class="mt-4">
			<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
			<select
				value={step.next || ''}
				onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
				class="w-full rounded-md border-gray-300 text-sm"
			>
				<option value="">-- End flow --</option>
				{#each getNextStepOptions() as s}
					<option value={s.id}>{s.name} ({s.type})</option>
				{/each}
			</select>
		</div>

	{:else if step.type === 'decision'}
		<!-- Outcomes -->
		<div class="space-y-3">
			<div class="flex justify-between items-center">
				<label class="text-sm font-medium text-gray-700">Outcomes</label>
				<button type="button" onclick={addOutcome} class="text-sm text-primary hover:text-blue-800">
					+ Add Outcome
				</button>
			</div>

			{#each step.outcomes as outcome, index (outcome.id)}
				<div class="border border-gray-200 rounded p-3 bg-gray-50 space-y-2">
					<div class="flex justify-between items-start">
						<input
							type="text"
							value={outcome.label}
							onchange={(e) => updateOutcome(index, { label: e.currentTarget.value })}
							placeholder="Outcome label"
							class="text-sm rounded-md border-gray-300 flex-1"
						/>
						<button
							type="button"
							onclick={() => removeOutcome(index)}
							class="text-red-600 hover:text-red-800 text-sm ml-2"
						>
							&times;
						</button>
					</div>
					<input
						type="text"
						value={outcome.condition}
						onchange={(e) => updateOutcome(index, { condition: e.currentTarget.value })}
						placeholder="Condition expression (e.g., score > 70)"
						class="w-full text-sm rounded-md border-gray-300 font-mono"
					/>
					<select
						value={outcome.next || ''}
						onchange={(e) => updateOutcome(index, { next: e.currentTarget.value || undefined })}
						class="w-full rounded-md border-gray-300 text-sm"
					>
						<option value="">-- End flow --</option>
						{#each getNextStepOptions() as s}
							<option value={s.id}>{s.name} ({s.type})</option>
						{/each}
					</select>
				</div>
			{/each}

			<div class="mt-4">
				<label class="block text-sm font-medium text-gray-700 mb-1">Default (if no outcome matches)</label>
				<select
					value={step.defaultNext || ''}
					onchange={(e) => onUpdate({ ...step, defaultNext: e.currentTarget.value || undefined })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">-- End flow --</option>
					{#each getNextStepOptions() as s}
						<option value={s.id}>{s.name} ({s.type})</option>
					{/each}
				</select>
			</div>
		</div>

	{:else if step.type === 'assignment'}
		<!-- Assignments -->
		<div class="space-y-3">
			<div class="flex justify-between items-center">
				<label class="text-sm font-medium text-gray-700">Assignments</label>
				<button type="button" onclick={addAssignment} class="text-sm text-primary hover:text-blue-800">
					+ Add Assignment
				</button>
			</div>

			{#each step.assignments as assignment, index}
				<div class="flex gap-2 items-center">
					<input
						type="text"
						value={assignment.variable}
						onchange={(e) => updateAssignment(index, { variable: e.currentTarget.value })}
						placeholder="Variable name"
						class="text-sm rounded-md border-gray-300 flex-1"
					/>
					<select
						value={assignment.operation}
						onchange={(e) => updateAssignment(index, { operation: e.currentTarget.value as 'set' | 'add' | 'subtract' })}
						class="text-sm rounded-md border-gray-300"
					>
						<option value="set">=</option>
						<option value="add">+=</option>
						<option value="subtract">-=</option>
					</select>
					<input
						type="text"
						value={assignment.value}
						onchange={(e) => updateAssignment(index, { value: e.currentTarget.value })}
						placeholder="Value or expression"
						class="text-sm rounded-md border-gray-300 flex-1 font-mono"
					/>
					<button
						type="button"
						onclick={() => removeAssignment(index)}
						class="text-red-600 hover:text-red-800"
					>
						&times;
					</button>
				</div>
			{/each}

			{#if step.assignments.length === 0}
				<p class="text-sm text-gray-500 text-center py-2">No assignments. Add one to set variable values.</p>
			{/if}
		</div>

		<!-- Next step -->
		<div class="mt-4">
			<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
			<select
				value={step.next || ''}
				onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
				class="w-full rounded-md border-gray-300 text-sm"
			>
				<option value="">-- End flow --</option>
				{#each getNextStepOptions() as s}
					<option value={s.id}>{s.name} ({s.type})</option>
				{/each}
			</select>
		</div>

	{:else if step.type === 'record_create'}
		<div class="space-y-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Entity</label>
				<select
					value={step.entity}
					onchange={(e) => onUpdate({ ...step, entity: e.currentTarget.value })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">Select entity</option>
					{#each entities as entity}
						<option value={entity.name}>{entity.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Field Mappings</label>
				<p class="text-xs text-gray-500 mb-2">Map entity fields to values or expressions</p>
				{#each fields.filter(f => f.name !== 'id' && f.name !== 'created_at' && f.name !== 'updated_at') as field}
					<div class="flex gap-2 mb-2">
						<span class="text-sm text-gray-600 w-32 truncate">{field.label}</span>
						<input
							type="text"
							value={step.fieldMappings[field.name] || ''}
							onchange={(e) => updateFieldMapping(field.name, e.currentTarget.value)}
							placeholder={'Value or {{variable}}'}
							class="text-sm rounded-md border-gray-300 flex-1 font-mono"
						/>
					</div>
				{/each}
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Store Result As (optional)</label>
				<input
					type="text"
					value={step.storeResultAs || ''}
					onchange={(e) => onUpdate({ ...step, storeResultAs: e.currentTarget.value || undefined })}
					placeholder="Variable name"
					class="w-full rounded-md border-gray-300 text-sm"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
				<select
					value={step.next || ''}
					onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">-- End flow --</option>
					{#each getNextStepOptions() as s}
						<option value={s.id}>{s.name} ({s.type})</option>
					{/each}
				</select>
			</div>
		</div>

	{:else if step.type === 'record_update'}
		<div class="space-y-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Entity</label>
				<select
					value={step.entity}
					onchange={(e) => onUpdate({ ...step, entity: e.currentTarget.value })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">Select entity</option>
					{#each entities as entity}
						<option value={entity.name}>{entity.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Record ID</label>
				<input
					type="text"
					value={step.recordId}
					onchange={(e) => onUpdate({ ...step, recordId: e.currentTarget.value })}
					placeholder={'{{$record.id}} or variable'}
					class="w-full rounded-md border-gray-300 text-sm font-mono"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Field Mappings</label>
				{#each fields.filter(f => f.name !== 'id' && f.name !== 'created_at' && f.name !== 'updated_at') as field}
					<div class="flex gap-2 mb-2">
						<span class="text-sm text-gray-600 w-32 truncate">{field.label}</span>
						<input
							type="text"
							value={step.fieldMappings[field.name] || ''}
							onchange={(e) => updateFieldMapping(field.name, e.currentTarget.value)}
							placeholder={'Value or {{variable}}'}
							class="text-sm rounded-md border-gray-300 flex-1 font-mono"
						/>
					</div>
				{/each}
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
				<select
					value={step.next || ''}
					onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">-- End flow --</option>
					{#each getNextStepOptions() as s}
						<option value={s.id}>{s.name} ({s.type})</option>
					{/each}
				</select>
			</div>
		</div>

	{:else if step.type === 'record_get'}
		<div class="space-y-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Entity</label>
				<select
					value={step.entity}
					onchange={(e) => onUpdate({ ...step, entity: e.currentTarget.value })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">Select entity</option>
					{#each entities as entity}
						<option value={entity.name}>{entity.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Record ID</label>
				<input
					type="text"
					value={step.recordId}
					onchange={(e) => onUpdate({ ...step, recordId: e.currentTarget.value })}
					placeholder={'{{$record.id}} or variable'}
					class="w-full rounded-md border-gray-300 text-sm font-mono"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Store Result As</label>
				<input
					type="text"
					value={step.storeResultAs}
					onchange={(e) => onUpdate({ ...step, storeResultAs: e.currentTarget.value })}
					placeholder="Variable name"
					class="w-full rounded-md border-gray-300 text-sm"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
				<select
					value={step.next || ''}
					onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">-- End flow --</option>
					{#each getNextStepOptions() as s}
						<option value={s.id}>{s.name} ({s.type})</option>
					{/each}
				</select>
			</div>
		</div>

	{:else if step.type === 'n8n_webhook'}
		<div class="space-y-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Webhook URL</label>
				<input
					type="url"
					value={step.url}
					onchange={(e) => onUpdate({ ...step, url: e.currentTarget.value })}
					placeholder="https://n8n.example.com/webhook/..."
					class="w-full rounded-md border-gray-300 text-sm"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Method</label>
				<select
					value={step.method || 'POST'}
					onchange={(e) => onUpdate({ ...step, method: e.currentTarget.value })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="POST">POST</option>
					<option value="GET">GET</option>
				</select>
			</div>

			<label class="flex items-center text-sm text-gray-600">
				<input
					type="checkbox"
					checked={step.waitForResponse || false}
					onchange={(e) => onUpdate({ ...step, waitForResponse: e.currentTarget.checked })}
					class="mr-2 rounded"
				/>
				Wait for response
			</label>

			{#if step.waitForResponse}
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Store Response As</label>
					<input
						type="text"
						value={step.storeResultAs || ''}
						onchange={(e) => onUpdate({ ...step, storeResultAs: e.currentTarget.value || undefined })}
						placeholder="Variable name"
						class="w-full rounded-md border-gray-300 text-sm"
					/>
				</div>
			{/if}

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Next Step</label>
				<select
					value={step.next || ''}
					onchange={(e) => onUpdate({ ...step, next: e.currentTarget.value || undefined })}
					class="w-full rounded-md border-gray-300 text-sm"
				>
					<option value="">-- End flow --</option>
					{#each getNextStepOptions() as s}
						<option value={s.id}>{s.name} ({s.type})</option>
					{/each}
				</select>
			</div>
		</div>

	{:else if step.type === 'end'}
		<div class="space-y-3">
			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Completion Message</label>
				<textarea
					value={step.message || ''}
					onchange={(e) => onUpdate({ ...step, message: e.currentTarget.value || undefined })}
					placeholder="Flow completed successfully!"
					rows={2}
					class="w-full rounded-md border-gray-300 text-sm"
				></textarea>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 mb-1">Redirect (optional)</label>
				<div class="grid grid-cols-2 gap-2">
					<select
						value={step.redirect?.entity || ''}
						onchange={(e) => {
							const entity = e.currentTarget.value;
							if (entity) {
								onUpdate({ ...step, redirect: { entity, recordId: step.redirect?.recordId || '\{\{$record.id\}\}' } });
							} else {
								const { redirect, ...rest } = step;
								onUpdate({ ...rest, type: 'end' });
							}
						}}
						class="rounded-md border-gray-300 text-sm"
					>
						<option value="">No redirect</option>
						{#each entities as entity}
							<option value={entity.name}>{entity.label}</option>
						{/each}
					</select>
					{#if step.redirect}
						<input
							type="text"
							value={step.redirect.recordId}
							onchange={(e) => onUpdate({ ...step, redirect: { ...step.redirect!, recordId: e.currentTarget.value } })}
							placeholder="Record ID"
							class="rounded-md border-gray-300 text-sm font-mono"
						/>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
