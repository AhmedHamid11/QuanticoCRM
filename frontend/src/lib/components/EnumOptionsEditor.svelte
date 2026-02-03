<script lang="ts">
	interface Props {
		options: string[];
		onOptionsChange: (options: string[]) => void;
		label?: string;
		required?: boolean;
	}

	let { options = $bindable(), onOptionsChange, label = 'Options', required = false }: Props = $props();

	let draggedOptionIndex = $state<number | null>(null);

	function addOption() {
		options = [...options, ''];
		onOptionsChange(options);
	}

	function removeOption(index: number) {
		options = options.filter((_, i) => i !== index);
		onOptionsChange(options);
	}

	function updateOption(index: number, value: string) {
		options = options.map((opt, i) => i === index ? value : opt);
		onOptionsChange(options);
	}

	function handleDragStart(index: number) {
		draggedOptionIndex = index;
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		if (draggedOptionIndex === null || draggedOptionIndex === index) return;

		const newOptions = [...options];
		const draggedItem = newOptions[draggedOptionIndex];
		newOptions.splice(draggedOptionIndex, 1);
		newOptions.splice(index, 0, draggedItem);
		options = newOptions;
		draggedOptionIndex = index;
		onOptionsChange(options);
	}

	function handleDragEnd() {
		draggedOptionIndex = null;
	}
</script>

<div>
	<label class="block text-sm font-medium text-gray-700 mb-1">
		{label} {#if required}<span class="text-red-500">*</span>{/if}
	</label>
	<p class="text-xs text-gray-500 mb-2">Drag to reorder</p>
	<div class="space-y-2">
		{#each options as option, i}
			<div
				class="flex gap-2 items-center {draggedOptionIndex === i ? 'opacity-50' : ''}"
				draggable="true"
				ondragstart={() => handleDragStart(i)}
				ondragover={(e) => handleDragOver(e, i)}
				ondragend={handleDragEnd}
			>
				<span
					class="cursor-grab text-gray-400 hover:text-gray-600 px-1 select-none"
					title="Drag to reorder"
				>
					&#x2807;
				</span>
				<input
					type="text"
					value={option}
					oninput={(e) => updateOption(i, e.currentTarget.value)}
					class="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:ring-primary focus:border-primary"
					placeholder="Option value"
				/>
				{#if options.length > 1}
					<button
						type="button"
						onclick={() => removeOption(i)}
						class="px-3 py-2 text-red-600 hover:bg-red-50 rounded-md"
					>
						Remove
					</button>
				{/if}
			</div>
		{/each}
	</div>
	<button
		type="button"
		onclick={addOption}
		class="mt-2 text-sm text-primary hover:text-blue-800"
	>
		+ Add Option
	</button>
</div>
