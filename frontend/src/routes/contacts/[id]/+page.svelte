<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';
	import { DetailSkeleton, ErrorDisplay } from '$lib/components/ui';
	import RelatedList from '$lib/components/RelatedList.svelte';
	import ActivitiesStream from '$lib/components/ActivitiesStream.svelte';
	import SectionRenderer from '$lib/components/SectionRenderer.svelte';
	import type { Contact } from '$lib/types/contact';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import type { LayoutDataV2, LayoutV2Response, LayoutSectionV2 } from '$lib/types/layout';
	import { parseLayoutData, getVisibleSections } from '$lib/types/layout';

	type TabId = 'details' | 'activities';
	let activeTab = $state<TabId>('details');
	let entityDef = $state<EntityDef | null>(null);

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
	let layout = $state<LayoutDataV2 | null>(null);
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

	// Get visible sections based on record data
	let visibleSections = $derived(() => {
		if (!layout) return [];
		return getVisibleSections(layout, recordData());
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

			// Load layout (may be v1, v2, or legacy section array format)
			try {
				const layoutResponse = await get<{ layoutData: string }>(
					'/entities/Contact/layouts/detail'
				);
				layout = parseLayoutData(layoutResponse.layoutData, fieldsData.map(f => f.name));
			} catch {
				// Default layout with all fields
				layout = parseLayoutData('[]', fieldsData.map(f => f.name));
			}
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

	function getFieldValue(fieldName: string): unknown {
		if (!contact) return null;

		if (isSystemField(fieldName)) {
			const key = fieldNameToKey(fieldName) as keyof Contact;
			return contact[key];
		} else {
			// Custom field - get from customFields
			return (contact as Contact & { customFields?: Record<string, unknown> }).customFields?.[fieldName];
		}
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
				return null;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="text-sm text-gray-500">
		<a href="/contacts" class="hover:text-gray-700">Contacts</a>
		<span class="mx-2">/</span>
		<span class="text-gray-900">{contact ? getFullName(contact) : 'Loading...'}</span>
	</nav>

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

		<!-- Tabs (only show if entity has activities enabled) -->
		{#if entityDef?.hasActivities}
			<div class="border-b border-gray-200">
				<nav class="-mb-px flex space-x-8">
					<button
						onclick={() => activeTab = 'details'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'details' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Details
					</button>
					<button
						onclick={() => activeTab = 'activities'}
						class="whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm {activeTab === 'activities' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
					>
						Activities
					</button>
				</nav>
			</div>
		{/if}

		{#if activeTab === 'details'}
			<!-- Dynamic Sections from Layout -->
			{#each visibleSections() as section (section.id)}
				<SectionRenderer
					{section}
					{fields}
					record={recordData()}
					formatValue={formatFieldValue}
					renderLink={getLinkInfo}
				/>
			{/each}

			<!-- Description (if exists and not in layout) -->
			{#if contact.description}
				<div class="bg-white shadow rounded-lg p-6">
					<h2 class="text-lg font-medium text-gray-900 mb-4">Description</h2>
					<p class="text-sm text-gray-900 whitespace-pre-wrap">{contact.description}</p>
				</div>
			{/if}

			<!-- Record Information -->
			<div class="bg-white shadow rounded-lg p-6">
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

			<!-- Related Lists -->
			{#if enabledRelatedLists.length > 0}
				{#each enabledRelatedLists as config (config.id)}
					<RelatedList
						{config}
						parentEntity="Contact"
						parentId={contact.id}
					/>
				{/each}
			{/if}
		{:else if activeTab === 'activities'}
			<!-- Activities Tab -->
			<ActivitiesStream
				parentEntity="Contact"
				parentId={contact.id}
				parentName={getFullName(contact)}
			/>
		{/if}
	{/if}
</div>
