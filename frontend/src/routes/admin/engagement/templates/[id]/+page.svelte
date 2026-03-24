<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, post, put } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { isFeatureEnabled } from '$lib/stores/navigation.svelte';

	interface EmailTemplate {
		id: string;
		orgId: string;
		name: string;
		subject: string;
		bodyHtml: string;
		bodyText: string;
		hasComplianceFooter: number;
		createdBy: string;
		createdAt: string;
		updatedAt: string;
	}

	interface GmailStatus {
		connected: boolean;
		gmailAddress?: string;
	}

	let templateId = $derived($page.params.id);
	let template = $state<EmailTemplate | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Editable fields
	let name = $state('');
	let subject = $state('');
	let bodyHtml = $state('');

	// Auto-save
	let saveStatus = $state<'idle' | 'saving' | 'saved' | 'error'>('idle');
	let saveTimer: ReturnType<typeof setTimeout> | null = null;

	// Gmail
	let gmailConnected = $state(false);
	let gmailAddress = $state('');

	// Send test email
	let showSendTestModal = $state(false);
	let sendingTest = $state(false);
	let testEmail = $state('');
	let testContactId = $state('');

	// Preview
	let previewSubject = $state('');
	let previewBody = $state('');
	let useServerPreview = $state(false);
	let serverContactId = $state('');
	let serverPreviewLoading = $state(false);

	// Default sample data for client-side preview
	const sampleData: Record<string, string> = {
		first_name: 'Jane',
		last_name: 'Doe',
		full_name: 'Jane Doe',
		email: 'jane.doe@acmecorp.com',
		phone: '+1 555-0100',
		company: 'Acme Corp',
		title: 'VP of Sales',
		city: 'New York',
		state: 'NY',
		country: 'USA'
	};

	// ---- Client-side template renderer (mirrors Go logic) ----
	function renderTemplate(tmpl: string, vars: Record<string, string>): string {
		let result = tmpl;
		// 1. Conditionals with else
		result = result.replace(
			/\{\{#if\s+(\w+)\s*==\s*"([^"]*)"\}\}([\s\S]*?)\{\{else\}\}([\s\S]*?)\{\{\/if\}\}/g,
			(_, field, value, trueBody, falseBody) => (vars[field] === value ? trueBody : falseBody)
		);
		// 2. Conditionals without else
		result = result.replace(
			/\{\{#if\s+(\w+)\s*==\s*"([^"]*)"\}\}([\s\S]*?)\{\{\/if\}\}/g,
			(_, field, value, trueBody) => (vars[field] === value ? trueBody : '')
		);
		// 3. Variables with fallback
		result = result.replace(/\{\{(\w+)\|([^}]*)\}\}/g, (_, key, fallback) => {
			const val = vars[key];
			return val && val.trim() ? val : fallback;
		});
		// 4. Plain variables
		result = result.replace(/\{\{(\w+)\}\}/g, (_, key) => vars[key] ?? '');
		return result;
	}

	// ---- Reactive preview ----
	$effect(() => {
		if (!useServerPreview) {
			previewSubject = renderTemplate(subject, sampleData);
			previewBody = renderTemplate(bodyHtml, sampleData);
		}
	});

	async function fetchServerPreview() {
		if (!templateId) return;
		serverPreviewLoading = true;
		try {
			const reqBody: Record<string, unknown> = serverContactId.trim()
				? { contactId: serverContactId.trim() }
				: { sampleData };
			const result = await post<{ subject: string; bodyHtml: string }>(
				`/gmail/email-templates/${templateId}/preview`,
				reqBody
			);
			previewSubject = result.subject;
			previewBody = result.bodyHtml;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Preview failed');
		} finally {
			serverPreviewLoading = false;
		}
	}

	// ---- Auto-save (1s debounce) ----
	function scheduleAutoSave() {
		if (saveTimer) clearTimeout(saveTimer);
		saveStatus = 'idle';
		saveTimer = setTimeout(doSave, 1000);
	}

	async function doSave() {
		if (!template) return;
		saveStatus = 'saving';
		try {
			await put(`/gmail/email-templates/${template.id}`, {
				name: name.trim() || template.name,
				subject,
				bodyHtml,
				bodyText: '',
				hasComplianceFooter: template.hasComplianceFooter
			});
			saveStatus = 'saved';
			setTimeout(() => {
				if (saveStatus === 'saved') saveStatus = 'idle';
			}, 2000);
		} catch (e) {
			saveStatus = 'error';
			toast.error(e instanceof Error ? e.message : 'Failed to save template');
		}
	}

	// ---- Send test email ----
	async function handleSendTest() {
		if (!testEmail.trim()) {
			toast.error('Recipient email is required');
			return;
		}
		sendingTest = true;
		try {
			const result = await post<{ success: boolean; message: string }>('/gmail/send-test', {
				templateId,
				toEmail: testEmail.trim(),
				contactId: testContactId.trim() || undefined
			});
			toast.success(result.message);
			showSendTestModal = false;
			testEmail = '';
			testContactId = '';
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to send test email');
		} finally {
			sendingTest = false;
		}
	}

	function openSendTestModal() {
		if (!gmailConnected) {
			toast.error('Connect your Gmail account first — go to Admin > Integrations > Gmail');
			return;
		}
		testEmail = gmailAddress;
		showSendTestModal = true;
	}

	// ---- Load ----
	onMount(async () => {
		if (!isFeatureEnabled('cadences')) {
			goto('/admin');
			return;
		}
		await Promise.all([loadTemplate(), loadGmailStatus()]);
	});

	onDestroy(() => {
		if (saveTimer) clearTimeout(saveTimer);
	});

	async function loadTemplate() {
		try {
			loading = true;
			error = null;
			template = await get<EmailTemplate>(`/gmail/email-templates/${templateId}`);
			name = template.name;
			subject = template.subject;
			bodyHtml = template.bodyHtml;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load template';
		} finally {
			loading = false;
		}
	}

	async function loadGmailStatus() {
		try {
			const status = await get<GmailStatus>('/gmail/status');
			gmailConnected = status.connected;
			gmailAddress = status.gmailAddress ?? '';
		} catch {
			gmailConnected = false;
		}
	}
</script>

<svelte:head>
	<title>{name || 'Email Template'} — Quantico CRM</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center h-screen text-gray-400">
		<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 mr-2"></div>
		Loading template...
	</div>
{:else if error}
	<div class="p-6 rounded-lg bg-red-50 border border-red-200 text-red-700 m-6">
		{error}
	</div>
{:else if template}
	<!-- Full-height split layout -->
	<div class="flex flex-col h-screen overflow-hidden bg-white">

		<!-- Top bar -->
		<header class="flex items-center gap-3 px-4 py-2.5 border-b bg-white shrink-0 min-h-[52px]">
			<button
				onclick={() => goto('/admin/engagement/templates')}
				class="text-sm text-gray-400 hover:text-gray-700 transition-colors"
			>
				← Templates
			</button>
			<span class="text-gray-200">/</span>
			<input
				type="text"
				bind:value={name}
				oninput={scheduleAutoSave}
				placeholder="Template name"
				class="flex-1 text-sm font-semibold text-gray-900 border-0 outline-none bg-transparent focus:ring-0 min-w-0"
			/>
			<div class="flex items-center gap-2 ml-auto shrink-0">
				{#if saveStatus === 'saving'}
					<span class="text-xs text-blue-500">Saving…</span>
				{:else if saveStatus === 'saved'}
					<span class="text-xs text-green-500">Saved</span>
				{:else if saveStatus === 'error'}
					<span class="text-xs text-red-500">Save failed</span>
				{/if}
				<button
					onclick={openSendTestModal}
					disabled={!gmailConnected}
					title={gmailConnected ? 'Send a test email' : 'Connect Gmail first'}
					class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium border transition-colors
						{gmailConnected
						? 'border-gray-300 text-gray-700 hover:bg-gray-50'
						: 'border-gray-200 text-gray-300 cursor-not-allowed bg-gray-50'}"
				>
					<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
							d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
					</svg>
					Send Test
				</button>
				<button
					onclick={doSave}
					class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
				>
					Save
				</button>
			</div>
		</header>

		<!-- Split pane: left = editor, right = preview -->
		<div class="flex flex-1 overflow-hidden divide-x divide-gray-200">

			<!-- ===== LEFT PANE: Editor ===== -->
			<div class="w-1/2 flex flex-col overflow-hidden">
				<div class="px-4 py-2 bg-gray-50 border-b shrink-0">
					<span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Editor</span>
				</div>
				<div class="flex-1 overflow-y-auto p-4 space-y-4">

					<!-- Subject -->
					<div>
						<label class="block text-xs font-medium text-gray-700 mb-1">Subject Line</label>
						<input
							type="text"
							bind:value={subject}
							oninput={scheduleAutoSave}
							placeholder="Hi {{first_name}}, quick question about {{company}}"
							class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm
								focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
						/>
					</div>

					<!-- Body HTML -->
					<div class="flex flex-col">
						<label class="block text-xs font-medium text-gray-700 mb-1">Body HTML</label>
						<textarea
							bind:value={bodyHtml}
							oninput={scheduleAutoSave}
							rows={18}
							placeholder="<p>Hi {"{{"} first_name|there {"}}"} ,</p>..."
							class="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono
								leading-relaxed resize-none focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
						></textarea>
					</div>

					<!-- Variable reference panel -->
					<details class="rounded-md border border-gray-200 overflow-hidden">
						<summary class="px-3 py-2 text-xs font-semibold text-gray-600 bg-gray-50 cursor-pointer select-none hover:bg-gray-100">
							Available Variables
						</summary>
						<div class="px-3 py-2 grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
							{#each [
								['{{first_name}}', 'First name'],
								['{{last_name}}', 'Last name'],
								['{{full_name}}', 'Full name'],
								['{{email}}', 'Email address'],
								['{{phone}}', 'Phone number'],
								['{{company}}', 'Company name'],
								['{{title}}', 'Job title'],
								['{{city}}', 'City'],
								['{{state}}', 'State / Region'],
								['{{country}}', 'Country'],
							] as [token, label]}
								<code class="text-blue-600 text-xs">{token}</code>
								<span class="text-gray-500">{label}</span>
							{/each}
						</div>
						<div class="px-3 pb-2 text-xs text-gray-400">
							Fallback syntax: <code class="text-blue-600 font-mono">{'{{first_name|there}}'}</code>
							renders "there" when first_name is empty.
						</div>
					</details>
				</div>
			</div>

			<!-- ===== RIGHT PANE: Preview ===== -->
			<div class="w-1/2 flex flex-col overflow-hidden">
				<div class="px-4 py-2 bg-gray-50 border-b shrink-0 flex items-center justify-between">
					<span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
						Preview
						<span class="text-gray-300 font-normal ml-1 capitalize">
							{useServerPreview ? '(server)' : '(live · Jane Doe sample)'}
						</span>
					</span>
					<div class="flex items-center gap-2">
						<label class="text-xs text-gray-500 flex items-center gap-1 cursor-pointer">
							<input type="checkbox" bind:checked={useServerPreview} class="rounded" />
							Server render
						</label>
						{#if useServerPreview}
							<input
								type="text"
								bind:value={serverContactId}
								placeholder="Contact ID"
								class="text-xs border border-gray-300 rounded px-2 py-1 w-32"
							/>
							<button
								onclick={fetchServerPreview}
								disabled={serverPreviewLoading}
								class="text-xs px-2 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
							>
								{serverPreviewLoading ? '...' : 'Render'}
							</button>
						{/if}
					</div>
				</div>

				<div class="flex-1 overflow-y-auto p-4 space-y-3">
					<!-- Subject preview -->
					{#if previewSubject}
						<div class="rounded-md border border-gray-200 bg-gray-50 px-3 py-2">
							<span class="block text-xs text-gray-400 mb-0.5">Subject:</span>
							<span class="text-sm text-gray-800">{previewSubject}</span>
						</div>
					{/if}

					<!-- Body preview -->
					{#if previewBody}
						<div class="rounded-md border border-gray-200 overflow-hidden shadow-sm">
							<iframe
								title="Email body preview"
								srcdoc={previewBody}
								class="w-full bg-white"
								style="height: 560px;"
								sandbox="allow-same-origin"
							></iframe>
						</div>
					{:else}
						<div class="flex items-center justify-center h-48 text-gray-300 text-sm">
							Start typing to see a live preview
						</div>
					{/if}
				</div>
			</div>

		</div>
	</div>
{/if}

<!-- Send Test Email Modal -->
{#if showSendTestModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		onclick={() => (showSendTestModal = false)}
		role="dialog"
		aria-modal="true"
		aria-label="Send test email"
	>
		<div
			class="bg-white rounded-xl shadow-xl p-6 w-full max-w-md mx-4"
			onclick={(e) => e.stopPropagation()}
			role="none"
		>
			<h2 class="text-lg font-semibold text-gray-900 mb-1">Send Test Email</h2>
			<p class="text-sm text-gray-500 mb-4">
				Renders this template and sends it so you can preview how it looks in your inbox.
			</p>

			<div class="space-y-4">
				<div>
					<label for="test-to-email" class="block text-sm font-medium text-gray-700 mb-1">
						Recipient <span class="text-red-500">*</span>
					</label>
					<input
						id="test-to-email"
						type="email"
						bind:value={testEmail}
						placeholder="recipient@example.com"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				<div>
					<label for="test-contact-id" class="block text-sm font-medium text-gray-700 mb-1">
						Contact ID
						<span class="text-gray-400 font-normal">(optional — for variable substitution)</span>
					</label>
					<input
						id="test-contact-id"
						type="text"
						bind:value={testContactId}
						placeholder="Leave empty to send with raw tokens"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
							focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				{#if gmailAddress}
					<p class="text-xs text-gray-400">
						From: <span class="font-medium text-gray-600">{gmailAddress}</span>
					</p>
				{/if}
			</div>

			<div class="flex justify-end gap-3 mt-6">
				<button
					onclick={() => (showSendTestModal = false)}
					class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					onclick={handleSendTest}
					disabled={sendingTest || !testEmail.trim()}
					class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white
						bg-blue-600 rounded-lg hover:bg-blue-700
						disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
				>
					{#if sendingTest}
						<svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
						Sending…
					{:else}
						Send Test
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
