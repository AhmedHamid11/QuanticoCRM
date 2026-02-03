<script lang="ts">
	import Spinner from './Spinner.svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		type?: 'button' | 'submit' | 'reset';
		variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
		size?: 'sm' | 'md' | 'lg';
		loading?: boolean;
		disabled?: boolean;
		onclick?: (e: MouseEvent) => void;
		class?: string;
		children: Snippet;
	}

	let {
		type = 'button',
		variant = 'primary',
		size = 'md',
		loading = false,
		disabled = false,
		onclick,
		class: className = '',
		children
	}: Props = $props();

	const baseClasses = 'inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

	const variantClasses: Record<string, string> = {
		primary: 'bg-primary text-black hover:bg-primary/90 focus:ring-primary',
		secondary: 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 focus:ring-primary',
		danger: 'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500',
		ghost: 'text-gray-700 hover:bg-gray-100 focus:ring-gray-500'
	};

	const sizeClasses: Record<string, string> = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2 text-sm',
		lg: 'px-6 py-3 text-base'
	};

	let spinnerColor = $derived<'blue' | 'white' | 'gray'>(variant === 'primary' || variant === 'danger' ? 'white' : 'gray');
</script>

<button
	{type}
	{onclick}
	disabled={disabled || loading}
	class="{baseClasses} {variantClasses[variant]} {sizeClasses[size]} {className}"
>
	{#if loading}
		<Spinner size="sm" color={spinnerColor} class="mr-2" />
	{/if}
	{@render children()}
</button>
