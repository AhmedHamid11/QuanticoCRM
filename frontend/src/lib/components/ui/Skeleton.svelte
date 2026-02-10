<script lang="ts">
	interface Props {
		variant?: 'text' | 'heading' | 'avatar' | 'input' | 'card' | 'button';
		width?: string;
		height?: string;
		rounded?: 'none' | 'sm' | 'md' | 'lg' | 'full';
		class?: string;
	}

	let {
		variant = 'text',
		width,
		height,
		rounded = 'md',
		class: className = ''
	}: Props = $props();

	// Default dimensions based on variant
	const variantDefaults: Record<string, { width: string; height: string; rounded: string }> = {
		text: { width: '100%', height: '1rem', rounded: 'rounded' },
		heading: { width: '60%', height: '1.5rem', rounded: 'rounded' },
		avatar: { width: '2.5rem', height: '2.5rem', rounded: 'rounded-full' },
		input: { width: '100%', height: '2.5rem', rounded: 'rounded-md' },
		card: { width: '100%', height: '6rem', rounded: 'rounded-lg' },
		button: { width: '5rem', height: '2.25rem', rounded: 'rounded-md' }
	};

	const roundedClasses: Record<string, string> = {
		none: 'rounded-none',
		sm: 'rounded-sm',
		md: 'rounded-md',
		lg: 'rounded-lg',
		full: 'rounded-full'
	};

	let defaults = $derived(variantDefaults[variant] || variantDefaults.text);
	let appliedRounded = $derived(rounded === 'md' && variant === 'avatar' ? 'rounded-full' : roundedClasses[rounded]);
</script>

<div
	class="animate-pulse bg-gray-200 {appliedRounded} {className}"
	style="width: {width || defaults.width}; height: {height || defaults.height};"
></div>
