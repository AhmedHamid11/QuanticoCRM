<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import LoginForm from '$lib/components/LoginForm.svelte';

	// Redirect if already authenticated
	$effect(() => {
		if (auth.isAuthenticated) {
			goto('/');
		}
	});
</script>

<svelte:head>
	<title>Login - Quantico CRM</title>
</svelte:head>

<LoginForm />
