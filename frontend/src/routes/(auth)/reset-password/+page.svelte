<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	let password = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let success = $state(false);
	let isSubmitting = $state(false);

	// Get token from URL
	let token = $derived($page.url.searchParams.get('token') || '');

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

		if (!token) {
			error = 'Invalid or missing reset token';
			return;
		}

		isSubmitting = true;

		try {
			const response = await fetch('http://localhost:8080/api/v1/auth/reset-password', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({ token, newPassword: password })
			});

			const data = await response.json();

			if (!response.ok) {
				throw new Error(data.error || 'Failed to reset password');
			}

			success = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reset password';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Reset Password - Quantico CRM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-background-light py-12 px-4 sm:px-6 lg:px-8">
	<div class="max-w-md w-full space-y-8">
		<div>
			<h1 class="text-center text-3xl font-bold">
				<span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span>
			</h1>
			<h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Set new password</h2>
			{#if !success}
				<p class="mt-2 text-center text-sm text-gray-600">
					Enter your new password below.
				</p>
			{/if}
		</div>

		{#if !token && !success}
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
						<p class="text-sm font-medium text-red-800">
							Invalid password reset link. Please request a new one.
						</p>
					</div>
				</div>
			</div>
			<div class="text-center">
				<a href="/forgot-password" class="font-medium text-primary hover:text-primary">
					Request new reset link
				</a>
			</div>
		{:else if success}
			<div class="rounded-md bg-green-50 p-4">
				<div class="flex">
					<div class="flex-shrink-0">
						<svg class="h-5 w-5 text-green-400" viewBox="0 0 20 20" fill="currentColor">
							<path
								fill-rule="evenodd"
								d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
								clip-rule="evenodd"
							/>
						</svg>
					</div>
					<div class="ml-3">
						<p class="text-sm font-medium text-green-800">
							Your password has been reset successfully!
						</p>
					</div>
				</div>
			</div>
			<div class="text-center">
				<a
					href="/login"
					class="inline-flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-black bg-primary hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
				>
					Sign in with new password
				</a>
			</div>
		{:else}
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
						<label for="password" class="block text-sm font-medium text-gray-700">New password</label>
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
							>Confirm new password</label
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
							Resetting password...
						{:else}
							Reset password
						{/if}
					</button>
				</div>

				<div class="text-center">
					<a href="/login" class="font-medium text-primary hover:text-primary">
						Back to sign in
					</a>
				</div>
			</form>
		{/if}
	</div>
</div>
