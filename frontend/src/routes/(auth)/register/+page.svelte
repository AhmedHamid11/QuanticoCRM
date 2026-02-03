<script lang="ts">
	import { goto } from '$app/navigation';
	import { register, auth } from '$lib/stores/auth.svelte';

	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let firstName = $state('');
	let lastName = $state('');
	let orgName = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	// Redirect if already authenticated
	$effect(() => {
		if (auth.isAuthenticated) {
			goto('/');
		}
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';

		// Validate passwords match
		if (password !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}

		// Validate password length
		if (password.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}

		isSubmitting = true;

		try {
			await register({
				email,
				password,
				firstName,
				lastName,
				orgName
			});
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Registration failed';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Register - Quantico CRM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-background-light py-12 px-4 sm:px-6 lg:px-8">
	<div class="max-w-md w-full space-y-8">
		<div>
			<h1 class="text-center text-3xl font-bold">
				<span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span>
			</h1>
			<h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Create your account</h2>
			<p class="mt-2 text-center text-sm text-gray-600">
				Already have an account?
				<a href="/login" class="font-medium text-primary hover:text-primary"> Sign in </a>
			</p>
		</div>

		<form class="mt-8 space-y-6" onsubmit={handleSubmit}>
			{#if error}
				<div class="rounded-md bg-red-50 p-4">
					<div class="flex">
						<div class="flex-shrink-0">
							<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
								<path
									fill-rule="evenodd"
									d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
									clip-rule="evenodd"
								/>
							</svg>
						</div>
						<div class="ml-3">
							<p class="text-sm font-medium text-red-800">{error}</p>
						</div>
					</div>
				</div>
			{/if}

			<div class="space-y-4">
				<div>
					<label for="orgName" class="block text-sm font-medium text-gray-700"
						>Organization Name</label
					>
					<input
						id="orgName"
						name="orgName"
						type="text"
						required
						bind:value={orgName}
						class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
						placeholder="Acme Corp"
					/>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="firstName" class="block text-sm font-medium text-gray-700">First Name</label
						>
						<input
							id="firstName"
							name="firstName"
							type="text"
							bind:value={firstName}
							class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
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
							class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
							placeholder="Doe"
						/>
					</div>
				</div>

				<div>
					<label for="email" class="block text-sm font-medium text-gray-700">Email address</label>
					<input
						id="email"
						name="email"
						type="email"
						autocomplete="email"
						required
						bind:value={email}
						class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
						placeholder="you@company.com"
					/>
				</div>

				<div>
					<label for="password" class="block text-sm font-medium text-gray-700">Password</label>
					<input
						id="password"
						name="password"
						type="password"
						autocomplete="new-password"
						required
						bind:value={password}
						class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
						placeholder="At least 8 characters"
					/>
				</div>

				<div>
					<label for="confirmPassword" class="block text-sm font-medium text-gray-700"
						>Confirm Password</label
					>
					<input
						id="confirmPassword"
						name="confirmPassword"
						type="password"
						autocomplete="new-password"
						required
						bind:value={confirmPassword}
						class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-primary focus:border-primary sm:text-sm"
						placeholder="Confirm your password"
					/>
				</div>
			</div>

			<div>
				<button
					type="submit"
					disabled={isSubmitting}
					class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-black bg-primary hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{#if isSubmitting}
						<svg
							class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
						>
							<circle
								class="opacity-25"
								cx="12"
								cy="12"
								r="10"
								stroke="currentColor"
								stroke-width="4"
							></circle>
							<path
								class="opacity-75"
								fill="currentColor"
								d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
							></path>
						</svg>
						Creating account...
					{:else}
						Create account
					{/if}
				</button>
			</div>

			<p class="text-xs text-center text-gray-500">
				By creating an account, you agree to our terms of service and privacy policy.
			</p>
		</form>
	</div>
</div>
