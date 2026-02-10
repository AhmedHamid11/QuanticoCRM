<script lang="ts">
	import { PUBLIC_API_URL } from '$env/static/public';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import type { AuthResponse } from '$lib/types/auth';

	const API_BASE = PUBLIC_API_URL || '/api/v1';

	// Get token from URL
	let token = $derived($page.url.searchParams.get('token') || '');

	// Form state
	let password = $state('');
	let confirmPassword = $state('');
	let firstName = $state('');
	let lastName = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	// Redirect if already authenticated
	$effect(() => {
		if (auth.isAuthenticated && !token) {
			goto('/');
		}
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';

		if (!token) {
			error = 'Invalid invitation link. Please request a new invitation.';
			return;
		}

		if (password !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}

		if (password.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}

		isSubmitting = true;

		try {
			const response = await fetch(`${API_BASE}/auth/accept-invite`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					token,
					password,
					firstName: firstName.trim() || undefined,
					lastName: lastName.trim() || undefined
				})
			});

			if (!response.ok) {
				const data = await response.json();
				throw new Error(data.error || 'Failed to accept invitation');
			}

			const authResponse: AuthResponse = await response.json();

			// Store auth state
			localStorage.setItem('quantico_auth', JSON.stringify({
				user: authResponse.user,
				currentOrg: authResponse.user.memberships.find(m => m.isDefault) || authResponse.user.memberships[0],
				accessToken: authResponse.accessToken,
				refreshToken: authResponse.refreshToken,
				expiresAt: authResponse.expiresAt
			}));

			// Redirect to home - will pick up auth from localStorage
			window.location.href = '/';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to accept invitation';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Accept Invitation - Quantico CRM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
	<div class="max-w-md w-full space-y-8">
		<div>
			<h1 class="text-center text-3xl font-bold">
				<span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span>
			</h1>
			<h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Accept Invitation</h2>
			<p class="mt-2 text-center text-sm text-gray-600">
				Set up your account to join the organization
			</p>
		</div>

		{#if !token}
			<div class="rounded-md bg-red-50 p-4">
				<div class="flex">
					<div class="flex-shrink-0">
						<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
						</svg>
					</div>
					<div class="ml-3">
						<p class="text-sm font-medium text-red-800">Invalid invitation link. Please request a new invitation from your administrator.</p>
					</div>
				</div>
			</div>
			<div class="text-center">
				<a href="/login" class="text-blue-600 hover:text-blue-600 font-medium">Go to login</a>
			</div>
		{:else}
			<form class="mt-8 space-y-6" onsubmit={handleSubmit}>
				{#if error}
					<div class="rounded-md bg-red-50 p-4">
						<div class="flex">
							<div class="flex-shrink-0">
								<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
									<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
								</svg>
							</div>
							<div class="ml-3">
								<p class="text-sm font-medium text-red-800">{error}</p>
							</div>
						</div>
					</div>
				{/if}

				<div class="space-y-4">
					<div class="grid grid-cols-2 gap-4">
						<div>
							<label for="firstName" class="block text-sm font-medium text-gray-700">First Name</label>
							<input
								id="firstName"
								name="firstName"
								type="text"
								bind:value={firstName}
								class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
								placeholder="John"
							/>
						</div>
						<div>
							<label for="lastName" class="block text-sm font-medium text-gray-700">Last Name</label>
							<input
								id="lastName"
								name="lastName"
								type="text"
								bind:value={lastName}
								class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
								placeholder="Doe"
							/>
						</div>
					</div>

					<div>
						<label for="password" class="block text-sm font-medium text-gray-700">Password</label>
						<input
							id="password"
							name="password"
							type="password"
							required
							minlength="8"
							bind:value={password}
							class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
							placeholder="At least 8 characters"
						/>
					</div>

					<div>
						<label for="confirmPassword" class="block text-sm font-medium text-gray-700">Confirm Password</label>
						<input
							id="confirmPassword"
							name="confirmPassword"
							type="password"
							required
							minlength="8"
							bind:value={confirmPassword}
							class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
							placeholder="Confirm your password"
						/>
					</div>
				</div>

				<div>
					<button
						type="submit"
						disabled={isSubmitting}
						class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-600/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{#if isSubmitting}
							<svg class="animate-spin -ml-1 mr-3 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							Creating account...
						{:else}
							Accept Invitation
						{/if}
					</button>
				</div>

				<p class="text-center text-sm text-gray-600">
					Already have an account?
					<a href="/login" class="font-medium text-blue-600 hover:text-blue-600">Sign in</a>
				</p>
			</form>
		{/if}
	</div>
</div>
