<script lang="ts">
	import { goto } from '$app/navigation';

	let email = $state('');
	let error = $state('');
	let success = $state('');
	let resetToken = $state('');
	let isSubmitting = $state(false);

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		success = '';
		resetToken = '';
		isSubmitting = true;

		try {
			const response = await fetch('http://localhost:8080/api/v1/auth/forgot-password', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({ email })
			});

			const data = await response.json();

			if (!response.ok) {
				throw new Error(data.error || 'Failed to request password reset');
			}

			success = data.message;

			// In development, show the reset token/URL
			if (data.resetToken) {
				resetToken = data.resetToken;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to request password reset';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<svelte:head>
	<title>Forgot Password - Quantico CRM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
	<div class="max-w-md w-full space-y-8">
		<div>
			<h1 class="text-center text-3xl font-bold">
				<span class="text-red-800">Quantico</span><span class="text-amber-600">CRM</span>
			</h1>
			<h2 class="mt-6 text-center text-2xl font-semibold text-gray-900">Reset your password</h2>
			<p class="mt-2 text-center text-sm text-gray-600">
				Enter your email address and we'll send you a link to reset your password.
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

			{#if success}
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
							<p class="text-sm font-medium text-green-800">{success}</p>
						</div>
					</div>
				</div>

				{#if resetToken}
					<div class="rounded-md bg-blue-50 p-4 border border-blue-200">
						<p class="text-sm text-blue-800 mb-2">
							<strong>Development mode:</strong> Use the link below to reset your password.
						</p>
						<a
							href="/reset-password?token={resetToken}"
							class="text-sm font-medium text-blue-600 hover:text-blue-600 break-all"
						>
							Reset Password Link
						</a>
					</div>
				{/if}
			{/if}

			{#if !success}
				<div class="space-y-4">
					<div>
						<label for="email" class="block text-sm font-medium text-gray-700">Email address</label>
						<input
							id="email"
							name="email"
							type="email"
							autocomplete="email"
							required
							bind:value={email}
							class="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 rounded-md placeholder-gray-400 text-gray-900 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
							placeholder="you@company.com"
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
							Sending...
						{:else}
							Send reset link
						{/if}
					</button>
				</div>
			{/if}

			<div class="text-center">
				<a href="/login" class="font-medium text-blue-600 hover:text-blue-600">
					Back to sign in
				</a>
			</div>
		</form>
	</div>
</div>
