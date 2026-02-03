<script lang="ts">
	import '../../app.css';
	import Toast from '$lib/components/Toast.svelte';

	let { children } = $props();
</script>

<!-- Auth pages use a minimal layout without navigation -->
{@render children()}

<Toast />
