<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, post, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	interface Sequence {
		id: string;
		name: string;
		description?: string;
		status: string;
		timezone: string;
		businessHoursStart?: string;
		businessHoursEnd?: string;
	}

	interface SequenceStep {
		id: string;
		sequenceId: string;
		stepNumber: number;
		stepType: string;
		delayDays: number;
		delayHours: number;
		templateId?: string;
		configJson?: string;
	}

	interface EmailTemplate {
		id: string;
		name: string;
		subject: string;
	}

	interface SuppressionRule {
		field: string;
		operator: string;
		value: string;
	}

	// Step config parsed from configJson
	interface StepConfig {
		linkedinSubType?: string;
		suggestedMessage?: string;
		script?: string;
		description?: string;
		continueWithoutCompleting?: boolean;
	}

	const TIMEZONES = [
		'America/New_York',
		'America/Chicago',
		'America/Denver',
		'America/Los_Angeles',
		'America/Phoenix',
		'America/Anchorage',
		'Pacific/Honolulu',
		'Europe/London',
		'Europe/Paris',
		'Europe/Berlin',
		'Asia/Tokyo',
		'Asia/Shanghai',
		'Asia/Kolkata',
		'Australia/Sydney'
	];

	const PREMADE_TEMPLATES = [
		{
			name: 'Recent Conversation',
			description: 'Follow up with a prospect from a recent conversation',
			steps: [
				{ stepType: 'email', delayDays: 0, delayHours: 0, config: {} },
				{ stepType: 'call', delayDays: 2, delayHours: 0, config: { script: 'Follow up on our recent conversation.' } },
				{ stepType: 'email', delayDays: 4, delayHours: 0, config: {} },
				{ stepType: 'linkedin', delayDays: 7, delayHours: 0, config: { linkedinSubType: 'connect' } }
			]
		},
		{
			name: 'Trade Show Follow Up',
			description: 'Nurture leads met at a trade show or event',
			steps: [
				{ stepType: 'email', delayDays: 1, delayHours: 0, config: {} },
				{ stepType: 'linkedin', delayDays: 3, delayHours: 0, config: { linkedinSubType: 'connect' } },
				{ stepType: 'call', delayDays: 7, delayHours: 0, config: { script: 'Hi, we met at the event last week...' } },
				{ stepType: 'email', delayDays: 14, delayHours: 0, config: {} }
			]
		},
		{
			name: 'Product or Demo Request',
			description: 'Engage prospects who requested a demo or product info',
			steps: [
				{ stepType: 'email', delayDays: 0, delayHours: 2, config: {} },
				{ stepType: 'call', delayDays: 1, delayHours: 0, config: { script: 'Calling to schedule your demo...' } },
				{ stepType: 'email', delayDays: 3, delayHours: 0, config: {} },
				{ stepType: 'linkedin', delayDays: 5, delayHours: 0, config: { linkedinSubType: 'message', suggestedMessage: 'Hi, I noticed you requested a demo...' } }
			]
		}
	];

	let sequenceId = $derived($page.params.id);

	let sequence = $state<Sequence | null>(null);
	let steps = $state<SequenceStep[]>([]);
	let emailTemplates = $state<EmailTemplate[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let activating = $state(false);
	let showAddStep = $state(false);

	// Local editable state for sequence settings
	let localName = $state('');
	let localDescription = $state('');
	let localTimezone = $state('America/New_York');
	let localBhStart = $state('09:00');
	let localBhEnd = $state('17:00');
	let suppressionRules = $state<SuppressionRule[]>([]);

	// Parsed step configs (keyed by stepId)
	let stepConfigs = $state<Record<string, StepConfig>>({});

	function parseStepConfig(step: SequenceStep): StepConfig {
		if (!step.configJson) return {};
		try {
			return JSON.parse(step.configJson) as StepConfig;
		} catch {
			return {};
		}
	}

	function buildStepConfigJson(config: StepConfig): string {
		return JSON.stringify(config);
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [seqData, templatesData] = await Promise.all([
				get<{ sequence: Sequence; steps: SequenceStep[] }>(`/sequences/${sequenceId}`),
				get<EmailTemplate[]>('/gmail/email-templates').catch(() => [] as EmailTemplate[])
			]);

			sequence = seqData.sequence;
			steps = seqData.steps;
			emailTemplates = templatesData;

			// Initialize local state
			localName = sequence.name;
			localDescription = sequence.description ?? '';
			localTimezone = sequence.timezone ?? 'America/New_York';
			localBhStart = sequence.businessHoursStart ?? '09:00';
			localBhEnd = sequence.businessHoursEnd ?? '17:00';

			// Parse configs
			const configs: Record<string, StepConfig> = {};
			for (const step of steps) {
				configs[step.id] = parseStepConfig(step);
			}
			stepConfigs = configs;

		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sequence';
		} finally {
			loading = false;
		}
	}

	async function saveSequence() {
		if (!sequence) return;
		saving = true;
		try {
			await put(`/sequences/${sequenceId}`, {
				name: localName,
				description: localDescription,
				timezone: localTimezone,
				businessHoursStart: localBhStart,
				businessHoursEnd: localBhEnd,
				suppressionRules
			});
			toast.success('Sequence saved');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save sequence');
		} finally {
			saving = false;
		}
	}

	async function activateSequence() {
		if (!sequence) return;
		activating = true;
		try {
			await saveSequence();
			await post(`/sequences/${sequenceId}/activate`, {});
			sequence = { ...sequence, status: 'active' };
			toast.success('Sequence activated');
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to activate sequence');
		} finally {
			activating = false;
		}
	}

	async function addStep(stepType: string) {
		showAddStep = false;
		const maxStep = steps.length > 0 ? Math.max(...steps.map((s) => s.stepNumber)) : 0;
		try {
			const newStep = await post<SequenceStep>(`/sequences/${sequenceId}/steps`, {
				stepNumber: maxStep + 1,
				stepType,
				delayDays: 1,
				delayHours: 0,
				configJson: '{}'
			});
			steps = [...steps, newStep];
			stepConfigs = { ...stepConfigs, [newStep.id]: {} };
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to add step');
		}
	}

	async function deleteStep(stepId: string, stepNum: number) {
		if (!confirm(`Remove Step ${stepNum}?`)) return;
		try {
			await del(`/sequences/${sequenceId}/steps/${stepId}`);
			steps = steps.filter((s) => s.id !== stepId);
			const configs = { ...stepConfigs };
			delete configs[stepId];
			stepConfigs = configs;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to remove step');
		}
	}

	async function saveStep(step: SequenceStep) {
		const config = stepConfigs[step.id] ?? {};
		try {
			await put(`/sequences/${sequenceId}/steps/${step.id}`, {
				stepNumber: step.stepNumber,
				stepType: step.stepType,
				delayDays: step.delayDays,
				delayHours: step.delayHours,
				templateId: step.templateId ?? '',
				configJson: buildStepConfigJson(config)
			});
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to save step');
		}
	}

	async function applyTemplate(tmpl: typeof PREMADE_TEMPLATES[0]) {
		if (!confirm(`Apply template "${tmpl.name}"? This will replace all current steps.`)) return;
		// Delete all existing steps
		for (const step of steps) {
			await del(`/sequences/${sequenceId}/steps/${step.id}`).catch(() => {});
		}
		steps = [];
		stepConfigs = {};

		// Create new steps from template
		for (let i = 0; i < tmpl.steps.length; i++) {
			const t = tmpl.steps[i];
			try {
				const newStep = await post<SequenceStep>(`/sequences/${sequenceId}/steps`, {
					stepNumber: i + 1,
					stepType: t.stepType,
					delayDays: t.delayDays,
					delayHours: t.delayHours,
					configJson: JSON.stringify(t.config)
				});
				steps = [...steps, newStep];
				stepConfigs = { ...stepConfigs, [newStep.id]: t.config as StepConfig };
			} catch (e) {
				toast.error(`Failed to create step ${i + 1}`);
			}
		}
		toast.success(`Applied template: ${tmpl.name}`);
	}

	function addSuppressionRule() {
		suppressionRules = [...suppressionRules, { field: '', operator: 'equals', value: '' }];
	}

	function removeSuppressionRule(index: number) {
		suppressionRules = suppressionRules.filter((_, i) => i !== index);
	}

	function getStepTypeIcon(stepType: string): string {
		switch (stepType) {
			case 'email': return 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z';
			case 'call': return 'M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z';
			case 'linkedin': return 'M16 8a6 6 0 016 6v7h-4v-7a2 2 0 00-2-2 2 2 0 00-2 2v7h-4v-7a6 6 0 016-6zM2 9h4v12H2z M4 6a2 2 0 100-4 2 2 0 000 4z';
			case 'custom': return 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z';
			default: return 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2';
		}
	}

	function getStepTypeLabel(stepType: string): string {
		switch (stepType) {
			case 'email': return 'Email';
			case 'call': return 'Call';
			case 'linkedin': return 'LinkedIn';
			case 'custom': return 'Custom Task';
			default: return stepType;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<svelte:head>
	<title>Sequence Builder</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center h-screen text-gray-400">
		<svg class="animate-spin h-6 w-6 mr-2" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
		</svg>
		Loading builder...
	</div>
{:else if error}
	<div class="p-6">
		<div class="rounded-md bg-red-50 p-4 text-sm text-red-700">
			{error}
			<button onclick={loadData} class="ml-2 underline hover:no-underline">Retry</button>
		</div>
	</div>
{:else if sequence}
	<!-- Header bar -->
	<div class="flex items-center justify-between px-6 py-3 border-b border-gray-200 bg-white sticky top-0 z-10">
		<div class="flex items-center gap-3">
			<button onclick={() => goto('/admin/engagement/sequences')} class="text-gray-400 hover:text-gray-600">
				<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
				</svg>
			</button>
			<input
				type="text"
				bind:value={localName}
				class="text-lg font-semibold text-gray-900 border-0 focus:ring-0 focus:outline-none bg-transparent min-w-0 w-64"
				placeholder="Sequence name"
			/>
			<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {sequence.status === 'active' ? 'bg-green-100 text-green-700' : sequence.status === 'paused' ? 'bg-yellow-100 text-yellow-700' : 'bg-gray-100 text-gray-600'}">
				{sequence.status.charAt(0).toUpperCase() + sequence.status.slice(1)}
			</span>
		</div>
		<div class="flex items-center gap-2">
			<button
				onclick={saveSequence}
				disabled={saving}
				class="px-3 py-1.5 text-sm border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 disabled:opacity-50"
			>
				{saving ? 'Saving...' : 'Save'}
			</button>
			{#if sequence.status === 'draft'}
				<button
					onclick={activateSequence}
					disabled={activating || steps.length === 0}
					class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title={steps.length === 0 ? 'Add at least one step before activating' : ''}
				>
					{activating ? 'Activating...' : 'Activate'}
				</button>
			{/if}
		</div>
	</div>

	<!-- Apollo-style split layout -->
	<div class="flex h-[calc(100vh-57px)]">
		<!-- Left panel: Settings + Suppression + Templates (40%) -->
		<div class="w-2/5 border-r border-gray-200 overflow-y-auto bg-white">
			<div class="p-5 space-y-6">

				<!-- Sequence Settings -->
				<section>
					<h2 class="text-sm font-semibold text-gray-900 mb-3">Sequence Settings</h2>
					<div class="space-y-3">
						<div>
							<label class="block text-xs font-medium text-gray-600 mb-1">Description</label>
							<textarea
								bind:value={localDescription}
								rows="2"
								class="w-full text-sm border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
								placeholder="Optional description"
							></textarea>
						</div>
						<div>
							<label class="block text-xs font-medium text-gray-600 mb-1">Timezone</label>
							<select
								bind:value={localTimezone}
								class="w-full text-sm border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
							>
								{#each TIMEZONES as tz}
									<option value={tz}>{tz}</option>
								{/each}
							</select>
						</div>
						<div class="grid grid-cols-2 gap-3">
							<div>
								<label class="block text-xs font-medium text-gray-600 mb-1">Business Hours Start</label>
								<input
									type="time"
									bind:value={localBhStart}
									class="w-full text-sm border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
								/>
							</div>
							<div>
								<label class="block text-xs font-medium text-gray-600 mb-1">Business Hours End</label>
								<input
									type="time"
									bind:value={localBhEnd}
									class="w-full text-sm border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
								/>
							</div>
						</div>
					</div>
				</section>

				<!-- Suppression Rules -->
				<section>
					<div class="flex items-center justify-between mb-3">
						<h2 class="text-sm font-semibold text-gray-900">Suppression Rules</h2>
						<button
							onclick={addSuppressionRule}
							class="text-xs text-blue-600 hover:text-blue-800 font-medium"
						>
							+ Add Rule
						</button>
					</div>
					{#if suppressionRules.length === 0}
						<p class="text-xs text-gray-400 italic">No suppression rules. Contacts will not be auto-suppressed from this sequence.</p>
					{:else}
						<div class="space-y-2">
							{#each suppressionRules as rule, i}
								<div class="flex items-center gap-1.5">
									<input
										type="text"
										bind:value={rule.field}
										placeholder="Field (e.g. status)"
										class="flex-1 text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500 min-w-0"
									/>
									<select
										bind:value={rule.operator}
										class="text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500"
									>
										<option value="equals">equals</option>
										<option value="not_equals">not equals</option>
										<option value="in">in</option>
									</select>
									<input
										type="text"
										bind:value={rule.value}
										placeholder="Value"
										class="flex-1 text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500 min-w-0"
									/>
									<button
										onclick={() => removeSuppressionRule(i)}
										class="text-red-400 hover:text-red-600 shrink-0"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
										</svg>
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</section>

				<!-- Pre-made Templates -->
				<section>
					<h2 class="text-sm font-semibold text-gray-900 mb-3">Pre-made Templates</h2>
					<div class="space-y-2">
						{#each PREMADE_TEMPLATES as tmpl}
							<button
								onclick={() => applyTemplate(tmpl)}
								class="w-full text-left p-3 border border-gray-200 rounded-lg hover:border-blue-300 hover:bg-blue-50 transition-colors"
							>
								<div class="text-sm font-medium text-gray-900">{tmpl.name}</div>
								<div class="text-xs text-gray-500 mt-0.5">{tmpl.description}</div>
								<div class="text-xs text-blue-600 mt-1">{tmpl.steps.length} steps</div>
							</button>
						{/each}
					</div>
				</section>

			</div>
		</div>

		<!-- Right panel: Step preview (60%) -->
		<div class="w-3/5 overflow-y-auto bg-gray-50">
			<div class="p-5">
				<h2 class="text-sm font-semibold text-gray-900 mb-4">Steps</h2>

				{#if steps.length === 0}
					<div class="text-center py-12 text-gray-400">
						<svg class="mx-auto h-10 w-10 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 4v16m8-8H4" />
						</svg>
						<p class="text-sm font-medium text-gray-600">No steps yet</p>
						<p class="text-xs mt-1">Add your first step or apply a template from the left panel.</p>
					</div>
				{:else}
					<div class="space-y-3">
						{#each steps as step, idx (step.id)}
							{@const config = stepConfigs[step.id] ?? {}}
							<div class="bg-white rounded-lg border border-gray-200 p-4 shadow-sm">
								<!-- Step header -->
								<div class="flex items-start justify-between mb-3">
									<div class="flex items-center gap-2">
										<span class="inline-flex items-center justify-center w-7 h-7 rounded-full bg-blue-100 text-blue-700 text-xs font-bold shrink-0">
											{step.stepNumber}
										</span>
										<div>
											<div class="flex items-center gap-1.5">
												<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getStepTypeIcon(step.stepType)} />
												</svg>
												<span class="text-sm font-medium text-gray-900">{getStepTypeLabel(step.stepType)}</span>
											</div>
											<div class="text-xs text-gray-400 mt-0.5">
												Wait {step.delayDays} day{step.delayDays !== 1 ? 's' : ''}
												{#if step.delayHours > 0} {step.delayHours} hour{step.delayHours !== 1 ? 's' : ''}{/if}
											</div>
										</div>
									</div>
									<button
										onclick={() => deleteStep(step.id, step.stepNumber)}
										class="text-gray-300 hover:text-red-500 transition-colors"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
										</svg>
									</button>
								</div>

								<!-- Delay configuration -->
								<div class="flex items-center gap-2 mb-3">
									<label class="text-xs text-gray-500 whitespace-nowrap">Wait</label>
									<input
										type="number"
										min="0"
										bind:value={step.delayDays}
										onchange={() => saveStep(step)}
										class="w-16 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-500"
									/>
									<span class="text-xs text-gray-500">days</span>
									<input
										type="number"
										min="0"
										max="23"
										bind:value={step.delayHours}
										onchange={() => saveStep(step)}
										class="w-16 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-500"
									/>
									<span class="text-xs text-gray-500">hours</span>
								</div>

								<!-- Step type specific fields -->
								{#if step.stepType === 'email'}
									<div class="space-y-2">
										<div>
											<label class="block text-xs font-medium text-gray-600 mb-1">Email Template</label>
											<select
												bind:value={step.templateId}
												onchange={() => saveStep(step)}
												class="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500"
											>
												<option value="">— Select template —</option>
												{#each emailTemplates as tmpl}
													<option value={tmpl.id}>{tmpl.name} {tmpl.subject ? `(${tmpl.subject})` : ''}</option>
												{/each}
											</select>
										</div>
									</div>

								{:else if step.stepType === 'call'}
									<div class="space-y-2">
										<div>
											<label class="block text-xs font-medium text-gray-600 mb-1">Script / Notes</label>
											<textarea
												rows="3"
												bind:value={config.script}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
												placeholder="Call script or notes..."
											></textarea>
										</div>
										<label class="flex items-center gap-2 text-xs text-gray-600 cursor-pointer">
											<input
												type="checkbox"
												bind:checked={config.continueWithoutCompleting}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
											/>
											Continue sequence without completing task
										</label>
									</div>

								{:else if step.stepType === 'linkedin'}
									<div class="space-y-2">
										<div>
											<label class="block text-xs font-medium text-gray-600 mb-1">Sub-type</label>
											<select
												bind:value={config.linkedinSubType}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500"
											>
												<option value="view_profile">View Profile</option>
												<option value="connect">Connect</option>
												<option value="message">Send Message</option>
												<option value="interact">Interact with Post</option>
											</select>
										</div>
										<div>
											<label class="block text-xs font-medium text-gray-600 mb-1">Suggested Message</label>
											<textarea
												rows="3"
												bind:value={config.suggestedMessage}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
												placeholder="Suggested messaging..."
											></textarea>
										</div>
										<label class="flex items-center gap-2 text-xs text-gray-600 cursor-pointer">
											<input
												type="checkbox"
												bind:checked={config.continueWithoutCompleting}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
											/>
											Continue sequence without completing task
										</label>
									</div>

								{:else if step.stepType === 'custom'}
									<div class="space-y-2">
										<div>
											<label class="block text-xs font-medium text-gray-600 mb-1">Task Description</label>
											<textarea
												rows="3"
												bind:value={config.description}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
												placeholder="Describe the custom task..."
											></textarea>
										</div>
										<label class="flex items-center gap-2 text-xs text-gray-600 cursor-pointer">
											<input
												type="checkbox"
												bind:checked={config.continueWithoutCompleting}
												onchange={() => { stepConfigs = { ...stepConfigs, [step.id]: config }; saveStep(step); }}
												class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
											/>
											Continue sequence without completing task
										</label>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}

				<!-- Add Step button -->
				<div class="mt-4">
					{#if showAddStep}
						<div class="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
							<div class="px-4 py-2 bg-gray-50 border-b border-gray-200 flex items-center justify-between">
								<span class="text-xs font-medium text-gray-600">Select step type</span>
								<button onclick={() => (showAddStep = false)} class="text-gray-400 hover:text-gray-600 text-xs">Cancel</button>
							</div>
							{#each [['email', 'Email'], ['call', 'Call Task'], ['linkedin', 'LinkedIn Task'], ['custom', 'Custom Task']] as [type, label]}
								<button
									onclick={() => addStep(type)}
									class="w-full flex items-center gap-3 px-4 py-3 text-sm text-gray-700 hover:bg-gray-50 transition-colors text-left"
								>
									<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getStepTypeIcon(type)} />
									</svg>
									{label}
								</button>
							{/each}
						</div>
					{:else}
						<button
							onclick={() => (showAddStep = true)}
							class="w-full py-2.5 border-2 border-dashed border-gray-300 rounded-lg text-sm text-gray-500 hover:border-blue-400 hover:text-blue-600 transition-colors"
						>
							+ Add Step
						</button>
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}
