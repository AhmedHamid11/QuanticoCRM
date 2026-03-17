<script lang="ts">
	import { onMount } from 'svelte';
	import { get, post, put, del } from '$lib/utils/api';
	import { toast } from '$lib/stores/toast.svelte';

	interface Mirror {
		id: string;
		orgId: string;
		name: string;
		targetEntity: string;
		uniqueKeyField: string;
		unmappedFieldMode: 'strict' | 'flexible';
		rateLimit: number;
		isActive: boolean;
		createdAt: string;
		updatedAt: string;
	}

	interface Entity {
		name: string;
		displayName?: string;
	}

	let mirrors = $state<Mirror[]>([]);
	let entities = $state<Entity[]>([]);
	let loading = $state(true);

	// Create modal state
	let showCreateModal = $state(false);
	let creating = $state(false);
	let newMirror = $state({
		name: '',
		targetEntity: '',
		uniqueKeyField: '',
		unmappedFieldMode: 'flexible' as 'strict' | 'flexible',
		rateLimit: 500
	});

	async function loadMirrors() {
		try {
			loading = true;
			const response = await get<Mirror[]>('/admin/mirrors');
			mirrors = response || [];
		} catch (err) {
			toast.error('Failed to load mirrors');
		} finally {
			loading = false;
		}
	}

	async function loadEntities() {
		try {
			const response = await get<Entity[]>('/admin/entities');
			entities = response || [];
		} catch (err) {
			toast.error('Failed to load entities');
		}
	}

	async function createMirror() {
		if (!newMirror.name.trim()) {
			toast.error('Mirror name is required');
			return;
		}
		if (!newMirror.targetEntity) {
			toast.error('Target entity is required');
			return;
		}
		if (!newMirror.uniqueKeyField.trim()) {
			toast.error('Unique key field is required');
			return;
		}

		try {
			creating = true;
			const response = await post<Mirror>('/admin/mirrors', {
				name: newMirror.name.trim(),
				targetEntity: newMirror.targetEntity,
				uniqueKeyField: newMirror.uniqueKeyField.trim(),
				unmappedFieldMode: newMirror.unmappedFieldMode,
				rateLimit: newMirror.rateLimit
			});

			mirrors = [...mirrors, response];
			showCreateModal = false;
			toast.success('Mirror created');

			// Reset form
			newMirror = {
				name: '',
				targetEntity: '',
				uniqueKeyField: '',
				unmappedFieldMode: 'flexible',
				rateLimit: 500
			};
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create mirror');
		} finally {
			creating = false;
		}
	}

	async function toggleActive(mirror: Mirror) {
		const backup = [...mirrors];
		mirrors = mirrors.map((m) => (m.id === mirror.id ? { ...m, isActive: !m.isActive } : m));

		try {
			await put(`/admin/mirrors/${mirror.id}`, { isActive: !mirror.isActive });
			toast.success(mirror.isActive ? 'Mirror deactivated' : 'Mirror activated');
		} catch (err) {
			mirrors = backup;
			toast.error('Failed to update mirror');
		}
	}

	async function deleteMirror(mirror: Mirror) {
		if (
			!confirm(
				`Are you sure you want to delete "${mirror.name}"? This will remove the mirror configuration but not affect existing records.`
			)
		)
			return;

		const backup = [...mirrors];
		mirrors = mirrors.filter((m) => m.id !== mirror.id);

		try {
			await del(`/admin/mirrors/${mirror.id}`);
			toast.success('Mirror deleted');
		} catch (err) {
			mirrors = backup;
			toast.error('Failed to delete mirror');
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function getEntityDisplayName(entityName: string): string {
		const entity = entities.find((e) => e.name === entityName);
		return entity?.displayName || entityName;
	}

	onMount(() => {
		loadMirrors();
		loadEntities();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Mirrors</h1>
			<p class="mt-1 text-sm text-gray-500">
				Manage schema contracts for external data ingestion
			</p>
		</div>
		<div class="flex items-center gap-3">
			<a
				href="/admin"
				class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
			>
				Back to Admin
			</a>
			<button
				onclick={() => (showCreateModal = true)}
				class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				Create Mirror
			</button>
		</div>
	</div>

	<!-- Mirrors List -->
	{#if loading}
		<div class="flex items-center justify-center py-12">
			<svg
				class="animate-spin h-8 w-8 text-blue-600"
				xmlns="http://www.w3.org/2000/svg"
				fill="none"
				viewBox="0 0 24 24"
			>
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
				></circle>
				<path
					class="opacity-75"
					fill="currentColor"
					d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
				></path>
			</svg>
		</div>
	{:else if mirrors.length === 0}
		<div class="text-center py-12 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
			<svg
				class="mx-auto h-12 w-12 text-gray-400"
				fill="none"
				stroke="currentColor"
				viewBox="0 0 24 24"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
				/>
			</svg>
			<h3 class="mt-2 text-sm font-medium text-gray-900">No mirrors configured</h3>
			<p class="mt-1 text-sm text-gray-500">
				Create your first mirror to start ingesting external data
			</p>
			<button
				onclick={() => (showCreateModal = true)}
				class="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-600/90"
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				Create Mirror
			</button>
		</div>
	{:else}
		<div class="crm-card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Mirror</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Status</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Unmapped Mode</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Rate Limit</th
						>
						<th
							scope="col"
							class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
							>Created</th
						>
						<th scope="col" class="relative px-6 py-3">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-200">
					{#each mirrors as mirror (mirror.id)}
						<tr class="hover:bg-gray-50 cursor-pointer" onclick={() => (window.location.href = `/admin/mirrors/${mirror.id}`)}>
							<td class="px-6 py-4">
								<div class="flex items-center">
									<div
										class="flex-shrink-0 h-10 w-10 rounded-full bg-violet-100 flex items-center justify-center"
									>
										<svg
											class="h-5 w-5 text-violet-600"
											fill="none"
											stroke="currentColor"
											viewBox="0 0 24 24"
										>
											<path
												stroke-linecap="round"
												stroke-linejoin="round"
												stroke-width="2"
												d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
											/>
										</svg>
									</div>
									<div class="ml-4">
										<div class="text-sm font-medium text-gray-900">{mirror.name}</div>
										<div class="text-sm text-gray-500">{getEntityDisplayName(mirror.targetEntity)}</div>
									</div>
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if mirror.isActive}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
									>
										Active
									</span>
								{:else}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
									>
										Inactive
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								{#if mirror.unmappedFieldMode === 'flexible'}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
									>
										Flexible
									</span>
								{:else}
									<span
										class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800"
									>
										Strict
									</span>
								{/if}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{mirror.rateLimit}/min
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(mirror.createdAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
								<button
									onclick={(e) => {
										e.stopPropagation();
										toggleActive(mirror);
									}}
									class="text-blue-600 hover:text-blue-900 mr-4"
								>
									{mirror.isActive ? 'Deactivate' : 'Activate'}
								</button>
								<button
									onclick={(e) => {
										e.stopPropagation();
										deleteMirror(mirror);
									}}
									class="text-red-600 hover:text-red-900"
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- Create Mirror Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-lg font-medium text-gray-900">Create Mirror</h3>
				<button onclick={() => (showCreateModal = false)} class="text-gray-400 hover:text-gray-500">
					<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				</button>
			</div>

			<form
				onsubmit={(e) => {
					e.preventDefault();
					createMirror();
				}}
				class="space-y-4"
			>
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700"
						>Mirror Name <span class="text-red-500">*</span></label
					>
					<input
						type="text"
						id="name"
						bind:value={newMirror.name}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						placeholder="e.g., Salesforce Contacts"
						required
					/>
					<p class="mt-1 text-xs text-gray-500">A descriptive name for this mirror</p>
				</div>

				<div>
					<label for="targetEntity" class="block text-sm font-medium text-gray-700"
						>Target Entity <span class="text-red-500">*</span></label
					>
					<select
						id="targetEntity"
						bind:value={newMirror.targetEntity}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						required
					>
						<option value="">Select entity...</option>
						{#each entities as entity}
							<option value={entity.name}>{entity.displayName || entity.name}</option>
						{/each}
					</select>
					<p class="mt-1 text-xs text-gray-500">The entity where records will be created</p>
				</div>

				<div>
					<label for="uniqueKeyField" class="block text-sm font-medium text-gray-700"
						>Unique Key Field <span class="text-red-500">*</span></label
					>
					<input
						type="text"
						id="uniqueKeyField"
						bind:value={newMirror.uniqueKeyField}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						placeholder="e.g., sf_id, external_id"
						required
					/>
					<p class="mt-1 text-xs text-gray-500">Field used to match external records</p>
				</div>

				<div>
					<label for="unmappedFieldMode" class="block text-sm font-medium text-gray-700"
						>Unmapped Field Mode</label
					>
					<select
						id="unmappedFieldMode"
						bind:value={newMirror.unmappedFieldMode}
						class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
					>
						<option value="flexible">Flexible - accept unknown fields with warning</option>
						<option value="strict">Strict - reject unknown fields</option>
					</select>
					<p class="mt-1 text-xs text-gray-500">How to handle fields not in the mirror schema</p>
				</div>

				<div>
					<label for="rateLimit" class="block text-sm font-medium text-gray-700">Rate Limit</label>
					<div class="mt-1 flex rounded-md shadow-sm">
						<input
							type="number"
							id="rateLimit"
							bind:value={newMirror.rateLimit}
							min="1"
							max="10000"
							class="block w-full rounded-l-md border-gray-300 focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
						/>
						<span
							class="inline-flex items-center px-3 rounded-r-md border border-l-0 border-gray-300 bg-gray-50 text-gray-500 text-sm"
						>
							/min
						</span>
					</div>
					<p class="mt-1 text-xs text-gray-500">Maximum requests per minute (default: 500)</p>
				</div>

				<div class="flex justify-end gap-3 pt-4">
					<button
						type="button"
						onclick={() => (showCreateModal = false)}
						class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={creating}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-600/90 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{creating ? 'Creating...' : 'Create Mirror'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
