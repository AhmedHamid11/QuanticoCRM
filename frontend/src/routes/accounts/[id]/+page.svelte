<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get, del } from '$lib/utils/api';
	import { addToast } from '$lib/stores/toast.svelte';
	import Bearing from '$lib/components/Bearing.svelte';
	import DetailPageAlertWrapper from '$lib/components/DetailPageAlertWrapper.svelte';
	import RecordDetailLayout from '$lib/components/RecordDetailLayout.svelte';
	import type { Account } from '$lib/types/account';
	import type { RelatedListConfig } from '$lib/types/related-list';
	import type { BearingWithStages } from '$lib/types/bearing';
	import type { EntityDef, FieldDef } from '$lib/types/admin';
	import type { LayoutDataV3, LayoutV3Response } from '$lib/types/layout';

	// System fields that exist as columns in the accounts table
	const SYSTEM_FIELDS = new Set([
		'name', 'website', 'emailAddress', 'phoneNumber', 'type', 'industry',
		'sicCode', 'billingAddressStreet', 'billingAddressCity', 'billingAddressState',
		'billingAddressCountry', 'billingAddressPostalCode', 'shippingAddressStreet',
		'shippingAddressCity', 'shippingAddressState', 'shippingAddressCountry',
		'shippingAddressPostalCode', 'description', 'stage', 'assignedUserId',
		'createdAt', 'modifiedAt', 'createdById', 'modifiedById', 'deleted'
	]);

	function isSystemField(fieldName: string): boolean {
		return SYSTEM_FIELDS.has(fieldName);
	}

	let account = $state<(Account & { customFields?: Record<string, unknown> }) | null>(null);
	let relatedListConfigs = $state<RelatedListConfig[]>([]);
	let bearings = $state<BearingWithStages[]>([]);
	let fields = $state<FieldDef[]>([]);
	let layout = $state<LayoutDataV3 | null>(null);
	let entityDef = $state<EntityDef | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let accountId = $derived($page.params.id);

	// Filter to only enabled configs, sorted by sortOrder
	let enabledRelatedLists = $derived(
		relatedListConfigs
			.filter((c) => c.enabled)
			.sort((a, b) => a.sortOrder - b.sortOrder)
	);

	// Get record data as a flat object for visibility evaluation
	let recordData = $derived(() => {
		if (!account) return {};
		const data: Record<string, unknown> = {};

		// Add system fields
		for (const fieldName of SYSTEM_FIELDS) {
			if (fieldName in account) {
				data[fieldName] = (account as unknown as Record<string, unknown>)[fieldName];
			}
		}

		// Add custom fields
		if (account.customFields) {
			for (const [key, value] of Object.entries(account.customFields)) {
				data[key] = value;
			}
		}

		// Add rollup and other dynamic fields from the account object
		// These are returned at the top level, not in customFields
		for (const field of fields) {
			if (!SYSTEM_FIELDS.has(field.name) && field.name in account && !(field.name in data)) {
				data[field.name] = (account as unknown as Record<string, unknown>)[field.name];
			}
		}

		return data;
	});

	async function loadAccount() {
		try {
			loading = true;
			error = null;
			const [accountData, configsData, bearingsData, fieldsData, entityDefData] = await Promise.all([
				get<Account>(`/accounts/${accountId}`),
				get<RelatedListConfig[]>(`/entities/Account/related-list-configs`).catch(() => []),
				get<BearingWithStages[]>(`/entities/Account/bearings`).catch(() => []),
				get<FieldDef[]>(`/entities/Account/fields`).catch(() => []),
				get<EntityDef>(`/entities/Account/def`).catch(() => null)
			]);
			account = accountData;
			relatedListConfigs = configsData;
			bearings = bearingsData;
			fields = fieldsData;
			entityDef = entityDefData;

			// Load V3 layout
			try {
				const layoutResponse = await get<LayoutV3Response>('/metadata/entities/Account/layouts/detail/v3');
				layout = layoutResponse.layout;
			} catch {
				layout = null;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load account';
			addToast(error, 'error');
		} finally {
			loading = false;
		}
	}

	async function deleteAccount() {
		if (!confirm('Are you sure you want to delete this account?')) return;

		try {
			await del(`/accounts/${accountId}`);
			addToast('Account deleted', 'success');
			goto('/accounts');
		} catch (e) {
			const message = e instanceof Error ? e.message : 'Failed to delete account';
			addToast(message, 'error');
		}
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

		// Format rollup fields with decimal places
		if (field.type === 'rollup' && field.rollupResultType === 'numeric' && typeof value === 'number') {
			const decimalPlaces = field.rollupDecimalPlaces ?? 2;
			return value.toFixed(decimalPlaces);
		}

		switch (field.type) {
			case 'bool':
				return value ? 'Yes' : 'No';
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			case 'link':
				if (field.linkEntity === 'User' && account) {
					const nameField = fieldName.replace(/Id$/, 'Name');
					const name = (account as unknown as Record<string, unknown>)[nameField];
					return name ? String(name) : '-';
				}
				return String(value);
			case 'text':
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
				if (field.linkEntity && account) {
					if (field.linkEntity === 'User') return null;
					const id = (account as unknown as Record<string, unknown>)[fieldName];
					if (!id) return null;
					const entityPath = field.linkEntity.toLowerCase() + 's';
					return { href: `/${entityPath}/${id}`, text: String(value) };
				}
				return null;
			default:
				return null;
		}
	}

	function handleBearingUpdate(fieldName: string, newValue: string) {
		if (account) {
			// Update local state optimistically
			if (isSystemField(fieldName)) {
				(account as unknown as Record<string, unknown>)[fieldName] = newValue;
			} else {
				if (!account.customFields) {
					account.customFields = {};
				}
				account.customFields[fieldName] = newValue;
			}
			account = { ...account }; // Trigger reactivity
		}
	}

	onMount(() => {
		loadAccount();
	});
</script>

{#if loading}
	<div class="text-center py-12 text-gray-500">Loading...</div>
{:else if error}
	<div class="text-center py-12">
		<p class="text-red-500 mb-4">{error}</p>
		<a href="/accounts" class="text-blue-600 hover:underline">Back to Accounts</a>
	</div>
{:else if !account}
	<div class="text-center py-12">
		<p class="text-gray-500 mb-4">Account not found</p>
		<a href="/accounts" class="text-blue-600 hover:underline">Back to Accounts</a>
	</div>
{:else if account}
	<div class="space-y-6">
		<!-- Header -->
		<div class="flex justify-between items-start">
			<div>
				<div class="flex items-center space-x-2 text-sm text-gray-500 mb-2">
					<a href="/accounts" class="hover:text-gray-700">Accounts</a>
					<span>/</span>
					<span>{account.name}</span>
				</div>
				<h1 class="text-2xl font-bold text-gray-900">{account.name}</h1>
				{#if account.industry}
					<p class="text-gray-500">{account.industry}</p>
				{/if}
			</div>
			<div class="flex space-x-3">
				<a
					href="/accounts/{account.id}/edit"
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-600/90"
				>
					Edit
				</a>
				<button
					onclick={deleteAccount}
					class="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700"
				>
					Delete
				</button>
			</div>
		</div>

		<!-- Duplicate alert banner -->
		<DetailPageAlertWrapper
			entityType="Account"
			recordId={account.id}
		/>

		<!-- V3 Layout -->
		{#if layout}
			<RecordDetailLayout
				{layout}
				{fields}
				record={recordData()}
				entityName="Account"
				recordId={account.id}
				{relatedListConfigs}
				formatValue={formatFieldValue}
				renderLink={getLinkInfo}
			>
				<!-- Bearings (Stage Progress Indicators) — passed as children snippet -->
				{#if bearings.length > 0}
					<div class="space-y-4">
						{#each bearings.toSorted((a, b) => a.displayOrder - b.displayOrder) as bearing (bearing.id)}
							{@const fieldName = bearing.sourcePicklist}
							{@const currentVal = isSystemField(fieldName)
								? (account as unknown as Record<string, unknown>)[fieldName]
								: account.customFields?.[fieldName]}
							<Bearing
								{bearing}
								currentValue={currentVal as string | null}
								recordId={account.id}
								entityType="Account"
								fieldName={fieldName}
								onUpdate={(newValue) => handleBearingUpdate(fieldName, newValue)}
							/>
						{/each}
					</div>
				{/if}

				<!-- System Info -->
				<div class="crm-card overflow-hidden">
					<div class="px-6 py-4 border-b border-gray-200">
						<h2 class="text-lg font-medium text-gray-900">System Information</h2>
					</div>
					<div class="px-6 py-4">
						<dl class="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
							<div>
								<dt class="text-sm font-medium text-gray-500">Created</dt>
								<dd class="mt-1 text-sm text-gray-900">
									{formatDate(account.createdAt)}
									{#if account.createdByName}
										<span class="text-gray-500"> by {account.createdByName}</span>
									{/if}
								</dd>
							</div>
							<div>
								<dt class="text-sm font-medium text-gray-500">Last Modified</dt>
								<dd class="mt-1 text-sm text-gray-900">
									{formatDate(account.modifiedAt)}
									{#if account.modifiedByName}
										<span class="text-gray-500"> by {account.modifiedByName}</span>
									{/if}
								</dd>
							</div>
							<div>
								<dt class="text-sm font-medium text-gray-500">ID</dt>
								<dd class="mt-1 text-sm text-gray-500 font-mono">{account.id}</dd>
							</div>
						</dl>
					</div>
				</div>
			</RecordDetailLayout>
		{:else}
			<!-- Fallback: no layout loaded -->
			<p class="text-gray-500 text-sm">Layout not available.</p>
		{/if}
	</div>
{/if}
