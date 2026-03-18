<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, put, post } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

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
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Editable fields
	let name = $state('');
	let subject = $state('');
	let bodyHtml = $state('');

	// Gmail connection status
	let gmailConnected = $state(false);
	let gmailAddress = $state('');

	// Send test email state
	let showSendTestModal = $state(false);
	let sendingTest = $state(false);
	let testEmail = $state('');
	let testContactId = $state('');

	onMount(async () => {
		await Promise.all([loadTemplate(), loadGmailStatus()]);
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

	async function handleSave() {
		if (!name.trim()) {
			toast.error('Template name is required');
			return;
		}
		if (!subject.trim()) {
			toast.error('Subject is required');
			return;
		}

		saving = true;
		try {
			const updated = await put<EmailTemplate>(`/gmail/email-templates/${templateId}`, {
				name: name.trim(),
				subject: subject.trim(),
				bodyHtml,
				bodyText: '',
				hasComplianceFooter: template?.hasComplianceFooter ?? 0
			});
			template = updated;
			toast.success('Template saved');
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to save template';
			toast.error(msg);
		} finally {
			saving = false;
		}
	}

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
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Failed to send test email';
			toast.error(msg);
		} finally {
			sendingTest = false;
		}
	}

	function openSendTestModal() {
		if (!gmailConnected) {
			toast.error('Connect your Gmail account first — go to Admin > Integrations > Gmail');
			return;
		}
		showSendTestModal = true;
		testEmail = gmailAddress;
	}
</script>

<svelte:head>
	<title>{name || 'Email Template'} — Quantico CRM</title>
</svelte:head>

<div class="p-6 max-w-4xl mx-auto">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-sm text-gray-500 mb-6">
		<button
			onclick={() => goto('/admin/engagement/templates')}
			class="hover:text-gray-700 hover:underline"
		>
			Email Templates
		</button>
		<span>/</span>
		<span class="text-gray-900 font-medium">{name || 'Loading...'}</span>
	</nav>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else if error}
		<div class="rounded-lg bg-red-50 border border-red-200 p-4 text-red-700">
			{error}
		</div>
	{:else if template}
		<!-- Header -->
		<div class="flex items-center justify-between mb-6">
			<div>
				<h1 class="text-xl font-semibold text-gray-900">{name || 'Untitled Template'}</h1>
				<p class="text-sm text-gray-500 mt-0.5">
					Last saved: {new Date(template.updatedAt).toLocaleString()}
				</p>
			</div>
			<div class="flex items-center gap-3">
				<!-- Send Test Email button -->
				<div class="relative group">
					<button
						onclick={openSendTestModal}
						disabled={!gmailConnected}
						class="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors
							{gmailConnected
							? 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'
							: 'bg-gray-100 border border-gray-200 text-gray-400 cursor-not-allowed'}"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
								d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
						</svg>
						Send Test Email
					</button>
					{#if !gmailConnected}
						<div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:block
							bg-gray-800 text-white text-xs rounded px-2 py-1 whitespace-nowrap z-10">
							Connect Gmail first
						</div>
					{/if}
				</div>

				<!-- Save button -->
				<button
					onclick={handleSave}
					disabled={saving}
					class="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium
						bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed
						transition-colors"
				>
					{#if saving}
						<svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
						Saving...
					{:else}
						Save
					{/if}
				</button>
			</div>
		</div>

		<!-- Gmail connection notice -->
		{#if !gmailConnected}
			<div class="mb-4 flex items-center gap-3 rounded-lg bg-amber-50 border border-amber-200 p-3 text-sm text-amber-800">
				<svg class="w-4 h-4 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
					<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
				</svg>
				Gmail not connected — test sends are disabled.
				<a href="/admin/integrations/gmail" class="font-medium underline hover:no-underline">Connect Gmail</a>
			</div>
		{/if}

		<!-- Editor form -->
		<div class="space-y-5 bg-white rounded-xl border border-gray-200 shadow-sm p-6">
			<!-- Name -->
			<div>
				<label for="template-name" class="block text-sm font-medium text-gray-700 mb-1">
					Template Name
				</label>
				<input
					id="template-name"
					type="text"
					bind:value={name}
					placeholder="e.g. Initial Outreach"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
				/>
			</div>

			<!-- Subject -->
			<div>
				<label for="template-subject" class="block text-sm font-medium text-gray-700 mb-1">
					Subject Line
				</label>
				<input
					id="template-subject"
					type="text"
					bind:value={subject}
					placeholder="e.g. Quick question about {{accountName}}"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
				/>
				<p class="mt-1 text-xs text-gray-400">
					Use &#123;&#123;firstName&#125;&#125;, &#123;&#123;lastName&#125;&#125;, &#123;&#123;accountName&#125;&#125; for personalization
				</p>
			</div>

			<!-- Body HTML -->
			<div>
				<label for="template-body" class="block text-sm font-medium text-gray-700 mb-1">
					Email Body (HTML)
				</label>
				<textarea
					id="template-body"
					bind:value={bodyHtml}
					rows={16}
					placeholder="<p>Hi {{firstName}},</p>&#10;&#10;<p>...</p>"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono leading-relaxed
						focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-y"
				></textarea>
				<p class="mt-1 text-xs text-gray-400">
					Supports HTML. Variables: &#123;&#123;firstName&#125;&#125;, &#123;&#123;lastName&#125;&#125;,
					&#123;&#123;fullName&#125;&#125;, &#123;&#123;emailAddress&#125;&#125;, &#123;&#123;phone&#125;&#125;,
					&#123;&#123;accountName&#125;&#125;
				</p>
			</div>
		</div>
	{/if}
</div>

<!-- Send Test Email Modal -->
{#if showSendTestModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
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
				Send this template to yourself to preview how it looks.
			</p>

			<div class="space-y-4">
				<div>
					<label for="test-to-email" class="block text-sm font-medium text-gray-700 mb-1">
						Send to <span class="text-red-500">*</span>
					</label>
					<input
						id="test-to-email"
						type="email"
						bind:value={testEmail}
						placeholder="recipient@example.com"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				<div>
					<label for="test-contact-id" class="block text-sm font-medium text-gray-700 mb-1">
						Contact ID <span class="text-gray-400 font-normal">(optional — for variable preview)</span>
					</label>
					<input
						id="test-contact-id"
						type="text"
						bind:value={testContactId}
						placeholder="Leave empty to send with raw tokens"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>

				{#if gmailAddress}
					<p class="text-xs text-gray-400">
						Will send from: <span class="font-medium text-gray-600">{gmailAddress}</span>
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
						Sending...
					{:else}
						Send Test
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
