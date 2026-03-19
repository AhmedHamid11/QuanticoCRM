<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { DetailSkeleton, ErrorDisplay } from '$lib/components/ui';
	import DetailPageAlertWrapper from '$lib/components/DetailPageAlertWrapper.svelte';
	import RecordDetailLayout from '$lib/components/RecordDetailLayout.svelte';
	import type { Contact } from '$lib/types/contact';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import type { LayoutDataV3, LayoutV3Response } from '$lib/types/layout';

	interface ActivityItem {
		type: 'call' | 'sms';
		occurredAt: string;
		disposition?: string;
		notes?: string;
		direction?: 'inbound' | 'outbound';
		body?: string;
		sequenceName?: string | null;
		source: 'sequence' | 'mirror';
	}

	let entityDef = $state<EntityDef | null>(null);
	let activities = $state<ActivityItem[]>([]);
	let activitiesLoading = $state(false);
	let activityTimelineOpen = $state(true);
	let expandedActivityBodies = $state<Set<number>>(new Set());

	// System fields that exist as columns in the contacts table
	const SYSTEM_FIELDS = new Set([
		'salutationName', 'firstName', 'lastName', 'emailAddress',
		'phoneNumber', 'phoneNumberType', 'doNotCall', 'description',
		'addressStreet', 'addressCity', 'addressState', 'addressCountry',
		'addressPostalCode', 'accountId', 'accountName', 'assignedUserId'
	]);

	let contactId = $derived($page.params.id);
	let contact = $state<Contact | null>(null);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV3 | null>(null);
	let relatedListConfigs = $state<RelatedListConfig[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let deleting = $state(false);

	// Filter to only enabled configs, sorted by sortOrder
	let enabledRelatedLists = $derived(
		relatedListConfigs
			.filter((c) => c.enabled)
			.sort((a, b) => a.sortOrder - b.sortOrder)
	);

	// Get record data as a flat object for visibility evaluation
	let recordData = $derived(() => {
		if (!contact) return {};
		const data: Record<string, unknown> = {};

		// Add system fields
		for (const fieldName of SYSTEM_FIELDS) {
			const key = fieldName as keyof Contact;
			if (key in contact) {
				data[fieldName] = contact[key];
			}
		}

		// Add custom fields
		const customFields = (contact as Contact & { customFields?: Record<string, unknown> }).customFields;
		if (customFields) {
			for (const [key, value] of Object.entries(customFields)) {
				data[key] = value;
			}
		}

		return data;
	});

	// Map field names to contact property keys (camelCase)
	function fieldNameToKey(fieldName: string): string {
		// Convert snake_case to camelCase
		return fieldName.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
	}

	function isSystemField(fieldName: string): boolean {
		const key = fieldNameToKey(fieldName);
		return SYSTEM_FIELDS.has(key);
	}

	async function loadData() {
		try {
			loading = true;
			error = null;

			const [contactData, fieldsData, configsData, entityDefData] = await Promise.all([
				get<Contact & { customFields?: Record<string, unknown> }>(`/contacts/${contactId}`),
				get<FieldDef[]>('/entities/Contact/fields'),
				get<RelatedListConfig[]>('/entities/Contact/related-list-configs').catch(() => []),
				get<EntityDef>('/entities/Contact/def').catch(() => null)
			]);

			contact = contactData;
			fields = fieldsData;
			relatedListConfigs = configsData;
			entityDef = entityDefData;

			// Load V3 layout
			try {
				const layoutResponse = await get<LayoutV3Response>('/metadata/entities/Contact/layouts/detail/v3');
				layout = layoutResponse.layout;
			} catch {
				layout = null;
			}

			// Load activity timeline (non-blocking)
			loadActivity(contactId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load contact';
		} finally {
			loading = false;
		}
	}

	async function deleteContact() {
		if (!confirm('Are you sure you want to delete this contact?')) return;

		try {
			deleting = true;
			await del(`/contacts/${contactId}`);
			toast.success('Contact deleted');
			goto('/contacts');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete contact';
			toast.error(message);
			deleting = false;
		}
	}

	function getFullName(c: Contact): string {
		const parts = [c.salutationName, c.firstName, c.lastName].filter(Boolean);
		return parts.join(' ');
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString();
	}

	function getFieldDef(fieldName: string): FieldDef | undefined {
		return fields.find(f => f.name === fieldName);
	}

	function formatFieldValue(fieldName: string, value: unknown): string {
		if (value === null || value === undefined || value === '') return '-';

		const field = getFieldDef(fieldName);
		if (!field) return String(value);

		switch (field.type) {
			case 'bool':
				return value ? 'Yes' : 'No';
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			case 'link':
				if (field.linkEntity === 'User' && contact) {
					const nameField = fieldName.replace(/Id$/, 'Name');
					const nameKey = fieldNameToKey(nameField) as keyof Contact;
					const name = contact[nameKey];
					return name ? String(name) : '-';
				}
				return String(value);
			default:
				return String(value);
		}
	}

	function getLinkInfo(fieldName: string, value: unknown): { href: string; text: string } | null {
		const field = getFieldDef(fieldName);
		if (!field || !value) return null;

		switch (field.type) {
			case 'email':
				return { href: `mailto:${value}`, text: String(value) };
			case 'phone':
				return { href: `tel:${value}`, text: String(value) };
			case 'url':
				return { href: String(value), text: String(value) };
			case 'link':
				if (field.linkEntity && contact) {
					// User links render as plain text (no clickable link)
					if (field.linkEntity === 'User') return null;

					// Get the ID value for the link field
					const key = fieldNameToKey(fieldName) as keyof Contact;
					const id = contact[key];
					if (!id) return null;

					// Get the display name
					const nameFieldName = fieldName.replace(/Id$/, 'Name');
					const nameKey = fieldNameToKey(nameFieldName) as keyof Contact;
					const displayName = contact[nameKey];

					const entityPath = field.linkEntity.toLowerCase() + 's';
					return { href: `/${entityPath}/${id}`, text: displayName ? String(displayName) : String(id) };
				}
				return null;
			default:
				// Fallback: detect *Id fields with a corresponding *Name field
				// This handles cases where field type metadata is missing 'link'
				if (fieldName.endsWith('Id') && contact) {
					const nameFieldName = fieldName.replace(/Id$/, 'Name');
					const nameKey = fieldNameToKey(nameFieldName) as keyof Contact;
					const displayName = contact[nameKey];
					if (displayName) {
						// Infer entity path from field name (e.g., accountId -> accounts)
						const entityName = fieldName.replace(/Id$/, '');
						const entityPath = entityName.toLowerCase() + 's';
						return { href: `/${entityPath}/${value}`, text: String(displayName) };
					}
				}
				return null;
		}
	}

	async function loadActivity(id: string) {
		activitiesLoading = true;
		try {
			const result = await get<ActivityItem[]>(`/contacts/${id}/activity`);
			activities = result ?? [];
		} catch {
			// Activity is non-critical; silently fail — empty state will render
			activities = [];
		} finally {
			activitiesLoading = false;
		}
	}

	function formatActivityTime(dateStr: string): string {
		const d = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - d.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
		if (diffDays === 0) return 'Today at ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
		if (diffDays === 1) return 'Yesterday at ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
		if (diffDays < 7) return `${diffDays} days ago`;
		return d.toLocaleDateString([], { month: 'short', day: 'numeric', year: diffDays > 365 ? 'numeric' : undefined });
	}

	function getDispositionColor(disposition: string | undefined): string {
		switch (disposition) {
			case 'Connected': return 'bg-green-100 text-green-800';
			case 'Voicemail': return 'bg-gray-100 text-gray-700';
			case 'No Answer': return 'bg-yellow-100 text-yellow-800';
			case 'Wrong Number':
			case 'Not Interested': return 'bg-red-100 text-red-800';
			default: return 'bg-gray-100 text-gray-600';
		}
	}

	function toggleActivityBody(index: number) {
		const next = new Set(expandedActivityBodies);
		if (next.has(index)) {
			next.delete(index);
		} else {
			next.add(index);
		}
		expandedActivityBodies = next;
	}

	onMount(() => {
		loadData();
	});

	// Reload data when contactId changes (e.g., navigating between contacts)
	$effect(() => {
		if (contactId) {
			loadData();
		}
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500">
		<a href="/contacts" class="hover:text-gray-700">Contacts</a>
		<span class="mx-2">/</span>
		<span class="text-gray-900">{contact ? getFullName(contact) : 'Loading...'}</span>
	</nav>

	<!-- Duplicate alert banner -->
	{#if contact}
		<DetailPageAlertWrapper
			entityType="Contact"
			recordId={contact.id}
		/>
	{/if}

	{#if loading}
		<DetailSkeleton sections={2} fieldsPerSection={4} />
	{:else if error}
		<ErrorDisplay message={error} onRetry={loadData} />
	{:else if contact}
		<!-- Header -->
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-4">
				<div class="w-16 h-16 rounded-full bg-blue-600 flex items-center justify-center text-white text-2xl font-bold">
					{contact.firstName?.charAt(0) || contact.lastName?.charAt(0) || '?'}
				</div>
				<div>
					<h1 class="text-2xl font-bold text-gray-900">{getFullName(contact)}</h1>
					{#if contact.emailAddress}
						<p class="text-gray-500">{contact.emailAddress}</p>
					{/if}
				</div>
			</div>
			<div class="flex gap-2">
				<a
					href="/contacts/{contactId}/edit"
					class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
				>
					Edit
				</a>
				<button
					onclick={deleteContact}
					disabled={deleting}
					class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-red-600 hover:bg-red-700 disabled:opacity-50"
				>
					{deleting ? 'Deleting...' : 'Delete'}
				</button>
			</div>
		</div>

		<!-- V3 Layout -->
		{#if layout}
			<RecordDetailLayout
				{layout}
				{fields}
				record={recordData()}
				entityName="Contact"
				recordId={contact.id}
				{relatedListConfigs}
				formatValue={formatFieldValue}
				renderLink={getLinkInfo}
			>
				<!-- Record Information (system info) as children snippet -->
				<div class="crm-card p-6">
					<h2 class="text-lg font-medium text-gray-900 mb-4">Record Information</h2>
					<dl class="grid grid-cols-2 gap-4">
						<div>
							<dt class="text-sm font-medium text-gray-500">Created</dt>
							<dd class="text-sm text-gray-900">
								{formatDate(contact.createdAt)}
								{#if contact.createdByName}
									<span class="text-gray-500"> by {contact.createdByName}</span>
								{/if}
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500">Last Modified</dt>
							<dd class="text-sm text-gray-900">
								{formatDate(contact.modifiedAt)}
								{#if contact.modifiedByName}
									<span class="text-gray-500"> by {contact.modifiedByName}</span>
								{/if}
							</dd>
						</div>
					</dl>
				</div>
			</RecordDetailLayout>
		{:else}
			<!-- Fallback: no layout loaded -->
			<p class="text-gray-500 text-sm">Layout not available.</p>
		{/if}

		<!-- Activity Timeline -->
		<div class="crm-card overflow-hidden">
			<!-- Section header (collapsible) -->
			<button
				onclick={() => (activityTimelineOpen = !activityTimelineOpen)}
				class="w-full flex items-center justify-between px-6 py-4 text-left hover:bg-gray-50 transition-colors"
			>
				<div class="flex items-center gap-2">
					<svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
					<h2 class="text-base font-medium text-gray-900">Activity Timeline</h2>
					{#if !activitiesLoading}
						<span class="text-xs text-gray-400 font-normal">({activities.length} item{activities.length !== 1 ? 's' : ''})</span>
					{/if}
				</div>
				<svg
					class="h-4 w-4 text-gray-400 transition-transform {activityTimelineOpen ? 'rotate-180' : ''}"
					fill="none" viewBox="0 0 24 24" stroke="currentColor"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
				</svg>
			</button>

			{#if activityTimelineOpen}
				<div class="px-6 pb-6">
					{#if activitiesLoading}
						<div class="flex items-center gap-2 py-8 text-gray-400 justify-center">
							<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
							</svg>
							<span class="text-sm">Loading activity...</span>
						</div>
					{:else if activities.length === 0}
						<div class="flex flex-col items-center py-10 text-gray-400">
							<svg class="h-10 w-10 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
							</svg>
							<p class="text-sm text-gray-500">No call or SMS activity yet</p>
						</div>
					{:else}
						<!-- Timeline list -->
						<div class="relative">
							<!-- Vertical timeline line -->
							<div class="absolute left-4 top-2 bottom-2 w-0.5 bg-gray-200" aria-hidden="true"></div>

							<ul class="space-y-4">
								{#each activities as activity, i}
									{@const isBodyExpanded = expandedActivityBodies.has(i)}
									<li class="relative pl-10">
										<!-- Timeline dot / icon -->
										<div class="absolute left-0 w-8 h-8 rounded-full {activity.type === 'call' ? 'bg-blue-100' : 'bg-purple-100'} flex items-center justify-center ring-2 ring-white">
											{#if activity.type === 'call'}
												<svg class="h-4 w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
												</svg>
											{:else}
												<svg class="h-4 w-4 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
												</svg>
											{/if}
										</div>

										<!-- Activity card -->
										<div class="bg-gray-50 rounded-lg p-3 border border-gray-100">
											<div class="flex items-start justify-between gap-2 flex-wrap">
												<div class="flex items-center gap-2 flex-wrap">
													<!-- Type label -->
													<span class="text-xs font-semibold text-gray-700 uppercase tracking-wide">
														{activity.type === 'call' ? 'Call' : 'SMS'}
													</span>

													<!-- Call: disposition badge -->
													{#if activity.type === 'call' && activity.disposition}
														<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium {getDispositionColor(activity.disposition)}">
															{activity.disposition}
														</span>
													{/if}

													<!-- SMS: direction badge -->
													{#if activity.type === 'sms' && activity.direction}
														<span class="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-700">
															{#if activity.direction === 'outbound'}
																<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18" />
																</svg>
																Outbound
															{:else}
																<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
																	<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
																</svg>
																Inbound
															{/if}
														</span>
													{/if}

													<!-- Source: External badge for mirror-sourced -->
													{#if activity.source === 'mirror'}
														<span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-orange-50 text-orange-700 border border-orange-200">
															External
														</span>
													{/if}
												</div>

												<!-- Timestamp -->
												<span class="text-xs text-gray-400 shrink-0">{formatActivityTime(activity.occurredAt)}</span>
											</div>

											<!-- Sequence name (if from sequence) -->
											{#if activity.sequenceName}
												<p class="text-xs text-gray-500 mt-1">via <span class="font-medium">{activity.sequenceName}</span></p>
											{/if}

											<!-- Call notes -->
											{#if activity.type === 'call' && activity.notes}
												<p class="text-sm text-gray-700 mt-1.5">{activity.notes}</p>
											{/if}

											<!-- SMS body (truncated with expand) -->
											{#if activity.type === 'sms' && activity.body}
												<div class="mt-1.5">
													{#if activity.body.length > 100 && !isBodyExpanded}
														<p class="text-sm text-gray-700">{activity.body.slice(0, 100)}<span class="text-gray-400">...</span></p>
														<button
															onclick={() => toggleActivityBody(i)}
															class="text-xs text-blue-600 hover:underline mt-0.5"
														>
															Show more
														</button>
													{:else}
														<p class="text-sm text-gray-700">{activity.body}</p>
														{#if activity.body.length > 100}
															<button
																onclick={() => toggleActivityBody(i)}
																class="text-xs text-blue-600 hover:underline mt-0.5"
															>
																Show less
															</button>
														{/if}
													{/if}
												</div>
											{/if}
										</div>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>
