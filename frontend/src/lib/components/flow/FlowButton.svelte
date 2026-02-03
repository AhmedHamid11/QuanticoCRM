<script lang="ts">
	import FlowRunner from './FlowRunner.svelte';

	interface Props {
		flowId: string;
		label?: string;
		entity: string;
		recordId: string;
		variant?: 'primary' | 'secondary' | 'ghost';
		size?: 'sm' | 'md' | 'lg';
		refreshOnComplete?: boolean;
	}

	let { flowId, label = 'Run Flow', entity, recordId, variant = 'primary', size = 'md', refreshOnComplete = false }: Props = $props();

	let showRunner = $state(false);

	function openFlow() {
		showRunner = true;
	}

	function closeFlow() {
		showRunner = false;
	}

	const sizeClasses = {
		sm: 'px-2.5 py-1.5 text-xs',
		md: 'px-3 py-2 text-sm',
		lg: 'px-4 py-2 text-base'
	};

	const variantClasses = {
		primary: 'bg-primary text-black hover:bg-primary/90 focus:ring-primary border-transparent',
		secondary: 'bg-white text-gray-700 hover:bg-gray-50 focus:ring-primary border-gray-300',
		ghost: 'bg-transparent text-gray-600 hover:bg-gray-100 focus:ring-gray-500 border-transparent'
	};
</script>

<button
	type="button"
	onclick={openFlow}
	class="inline-flex items-center font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-offset-2 {sizeClasses[size]} {variantClasses[variant]}"
>
	<svg class="w-4 h-4 mr-1.5 -ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
	</svg>
	{label}
</button>

{#if showRunner}
	<FlowRunner {flowId} {entity} {recordId} {refreshOnComplete} onClose={closeFlow} />
{/if}
