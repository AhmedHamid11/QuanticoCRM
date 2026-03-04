<script lang="ts">
	import type { FieldDef } from '$lib/types/admin';
	import {
		formatCurrency,
		formatRelativeDate,
		formatDateTooltip,
		getEnumColor,
		parseMultiEnumValue
	} from '$lib/utils/fieldFormatters';

	interface Props {
		field: FieldDef;
		value: unknown;
		renderLink?: (fieldName: string, value: unknown) => { href: string; text: string } | null;
		isEditable?: boolean;
		onclick?: () => void;
	}

	let { field, value, renderLink, isEditable = false, onclick }: Props = $props();

	// Clipboard copy state (shows feedback on copy)
	let copied = $state(false);

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text).then(() => {
			copied = true;
			setTimeout(() => {
				copied = false;
			}, 1500);
		});
	}

	// Determine if value is empty
	function isEmpty(v: unknown): boolean {
		return v === null || v === undefined || v === '';
	}

	// Format int with thousands separators
	function formatInt(v: unknown): string {
		const n = Number(v);
		if (isNaN(n)) return String(v);
		return new Intl.NumberFormat('en-US').format(n);
	}

	// Format float with 2 decimal places
	function formatFloat(v: unknown): string {
		const n = Number(v);
		if (isNaN(n)) return String(v);
		return new Intl.NumberFormat('en-US', {
			minimumFractionDigits: 2,
			maximumFractionDigits: 2
		}).format(n);
	}
</script>

{#if isEmpty(value)}
	<!-- Empty / null value -->
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50 text-gray-400"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			—
		</div>
	{:else}
		<span class="text-gray-400">—</span>
	{/if}
{:else if field.type === 'email'}
	<div
		class="group inline-flex items-center gap-1 {isEditable ? 'cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50' : ''}"
		role={isEditable ? 'button' : undefined}
		tabindex={isEditable ? 0 : undefined}
		onclick={isEditable ? onclick : undefined}
		onkeydown={isEditable ? (e) => e.key === 'Enter' && onclick?.() : undefined}
	>
		<a
			href="mailto:{value}"
			class="text-blue-600 hover:underline text-sm"
			onclick={(e) => e.stopPropagation()}
		>{String(value)}</a>
		<button
			type="button"
			class="opacity-0 group-hover:opacity-100 transition-opacity ml-1 text-gray-400 hover:text-gray-600"
			title={copied ? 'Copied!' : 'Copy to clipboard'}
			onclick={(e) => { e.stopPropagation(); copyToClipboard(String(value)); }}
		>
			<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
			</svg>
		</button>
	</div>

{:else if field.type === 'phone'}
	<div
		class="group inline-flex items-center gap-1 {isEditable ? 'cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50' : ''}"
		role={isEditable ? 'button' : undefined}
		tabindex={isEditable ? 0 : undefined}
		onclick={isEditable ? onclick : undefined}
		onkeydown={isEditable ? (e) => e.key === 'Enter' && onclick?.() : undefined}
	>
		<a
			href="tel:{value}"
			class="text-blue-600 hover:underline text-sm"
			onclick={(e) => e.stopPropagation()}
		>{String(value)}</a>
		<button
			type="button"
			class="opacity-0 group-hover:opacity-100 transition-opacity ml-1 text-gray-400 hover:text-gray-600"
			title={copied ? 'Copied!' : 'Copy to clipboard'}
			onclick={(e) => { e.stopPropagation(); copyToClipboard(String(value)); }}
		>
			<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
			</svg>
		</button>
	</div>

{:else if field.type === 'url'}
	<div
		class="group inline-flex items-center gap-1 {isEditable ? 'cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50' : ''}"
		role={isEditable ? 'button' : undefined}
		tabindex={isEditable ? 0 : undefined}
		onclick={isEditable ? onclick : undefined}
		onkeydown={isEditable ? (e) => e.key === 'Enter' && onclick?.() : undefined}
	>
		<a
			href={String(value)}
			target="_blank"
			rel="noopener noreferrer"
			class="text-blue-600 hover:underline text-sm truncate max-w-xs"
			onclick={(e) => e.stopPropagation()}
		>{String(value)}</a>
		<button
			type="button"
			class="opacity-0 group-hover:opacity-100 transition-opacity ml-1 text-gray-400 hover:text-gray-600 flex-shrink-0"
			title={copied ? 'Copied!' : 'Copy to clipboard'}
			onclick={(e) => { e.stopPropagation(); copyToClipboard(String(value)); }}
		>
			<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
			</svg>
		</button>
	</div>

{:else if field.type === 'enum'}
	{@const color = getEnumColor(String(value))}
	<div
		class="{isEditable ? 'cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50 inline-block' : 'inline-block'}"
		role={isEditable ? 'button' : undefined}
		tabindex={isEditable ? 0 : undefined}
		onclick={isEditable ? onclick : undefined}
		onkeydown={isEditable ? (e) => e.key === 'Enter' && onclick?.() : undefined}
	>
		<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {color.bg} {color.text}">
			{String(value)}
		</span>
	</div>

{:else if field.type === 'multiEnum'}
	{@const items = parseMultiEnumValue(value)}
	<div
		class="{isEditable ? 'cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50' : ''}"
		role={isEditable ? 'button' : undefined}
		tabindex={isEditable ? 0 : undefined}
		onclick={isEditable ? onclick : undefined}
		onkeydown={isEditable ? (e) => e.key === 'Enter' && onclick?.() : undefined}
	>
		{#if items.length === 0}
			<span class="text-gray-400">—</span>
		{:else}
			<div class="flex flex-wrap gap-1">
				{#each items as item}
					{@const color = getEnumColor(item)}
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {color.bg} {color.text}">
						{item}
					</span>
				{/each}
			</div>
		{/if}
	</div>

{:else if field.type === 'bool'}
	{@const checked = value === true || value === 'true' || value === 1}
	<button
		type="button"
		role="switch"
		aria-checked={checked}
		disabled={!isEditable}
		class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 {checked ? 'bg-blue-600' : 'bg-gray-200'} {!isEditable ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}"
		onclick={isEditable ? onclick : undefined}
	>
		<span
			class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {checked ? 'translate-x-4' : 'translate-x-0.5'}"
		></span>
	</button>

{:else if field.type === 'currency'}
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{formatCurrency(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{formatCurrency(value)}</span>
	{/if}

{:else if field.type === 'date'}
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm" title={formatDateTooltip(String(value), false)}>
				{formatRelativeDate(String(value), 7)}
			</span>
		</div>
	{:else}
		<span class="text-sm" title={formatDateTooltip(String(value), false)}>
			{formatRelativeDate(String(value), 7)}
		</span>
	{/if}

{:else if field.type === 'datetime'}
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm" title={formatDateTooltip(String(value), true)}>
				{formatRelativeDate(String(value), 7)}
			</span>
		</div>
	{:else}
		<span class="text-sm" title={formatDateTooltip(String(value), true)}>
			{formatRelativeDate(String(value), 7)}
		</span>
	{/if}

{:else if field.type === 'int'}
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{formatInt(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{formatInt(value)}</span>
	{/if}

{:else if field.type === 'float'}
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{formatFloat(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{formatFloat(value)}</span>
	{/if}

{:else if field.type === 'address'}
	<!-- Address is a compound field; SectionRenderer groups sub-fields in Plan 02. Show as plain text for now. -->
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{String(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{String(value)}</span>
	{/if}

{:else if field.type === 'link' || field.type === 'linkMultiple'}
	{@const linked = renderLink ? renderLink(field.name, value) : null}
	{#if linked}
		{#if isEditable}
			<div
				class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50 inline-block"
				role="button"
				tabindex="0"
				onclick={onclick}
				onkeydown={(e) => e.key === 'Enter' && onclick?.()}
			>
				<a
					href={linked.href}
					class="text-blue-600 hover:underline text-sm"
					onclick={(e) => e.stopPropagation()}
				>{linked.text}</a>
			</div>
		{:else}
			<a href={linked.href} class="text-blue-600 hover:underline text-sm">{linked.text}</a>
		{/if}
	{:else if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{String(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{String(value)}</span>
	{/if}

{:else if field.type === 'rollup'}
	{@const isNumeric = field.rollupResultType === 'numeric'}
	{#if isEditable}
		<span class="text-sm">{isNumeric ? formatFloat(value) : String(value)}</span>
	{:else}
		<span class="text-sm">{isNumeric ? formatFloat(value) : String(value)}</span>
	{/if}

{:else}
	<!-- varchar, text, textBlock, stream, and any unknown type — plain text -->
	{#if isEditable}
		<div
			class="cursor-pointer rounded px-1 -mx-1 transition-colors hover:bg-gray-50"
			role="button"
			tabindex="0"
			{onclick}
			onkeydown={(e) => e.key === 'Enter' && onclick?.()}
		>
			<span class="text-sm">{String(value)}</span>
		</div>
	{:else}
		<span class="text-sm">{String(value)}</span>
	{/if}
{/if}
