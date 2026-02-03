<script lang="ts">
	interface Props {
		size?: 'sm' | 'md' | 'lg';
		color?: 'blue' | 'white' | 'gray';
		class?: string;
	}

	let { size = 'md', color = 'blue', class: className = '' }: Props = $props();

	const sizeClasses: Record<string, string> = {
		sm: 'h-4 w-4 border-2',
		md: 'h-6 w-6 border-2',
		lg: 'h-10 w-10 border-3'
	};

	const colorClasses: Record<string, string> = {
		blue: 'border-blue-600 border-t-transparent',
		white: 'border-white border-t-transparent',
		gray: 'border-gray-400 border-t-transparent'
	};
</script>

<div
	class="animate-spin rounded-full {sizeClasses[size]} {colorClasses[color]} {className}"
	role="status"
	aria-label="Loading"
></div>
