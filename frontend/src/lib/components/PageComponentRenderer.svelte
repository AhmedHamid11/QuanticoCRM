<script lang="ts">
	import type { PageComponent, IframeConfig, TextConfig, HTMLConfig, EntityListConfig, LinkGroupConfig, StatsConfig } from '$lib/types/custom-page';
	import { marked } from 'marked';

	interface Props {
		component: PageComponent;
	}

	let { component }: Props = $props();

	// Type guards for config
	function isIframeConfig(config: unknown): config is IframeConfig {
		return component.type === 'iframe';
	}

	function isTextConfig(config: unknown): config is TextConfig {
		return component.type === 'text' || component.type === 'markdown';
	}

	function isHTMLConfig(config: unknown): config is HTMLConfig {
		return component.type === 'html';
	}

	function isEntityListConfig(config: unknown): config is EntityListConfig {
		return component.type === 'entity_list';
	}

	function isLinkGroupConfig(config: unknown): config is LinkGroupConfig {
		return component.type === 'link_group';
	}

	function isStatsConfig(config: unknown): config is StatsConfig {
		return component.type === 'stats';
	}

	// Render markdown to HTML
	function renderMarkdown(content: string): string {
		try {
			return marked(content) as string;
		} catch {
			return content;
		}
	}

	// Get icon SVG (simplified icon set)
	function getIconPath(iconName: string): string {
		const icons: Record<string, string> = {
			'globe': 'M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9',
			'link': 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1',
			'external-link': 'M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14',
			'users': 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z',
			'chart': 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
			'bar-chart': 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
			'trending-up': 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6',
			'dollar-sign': 'M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
			'clock': 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
			'calendar': 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
			'star': 'M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z',
			'check': 'M5 13l4 4L19 7',
		};
		return icons[iconName] || icons['star'];
	}

	// Get stat color class
	function getStatColorClass(color: string | undefined): string {
		const colors: Record<string, string> = {
			'blue': 'bg-blue-100 text-primary',
			'green': 'bg-green-100 text-green-600',
			'red': 'bg-red-100 text-red-600',
			'yellow': 'bg-yellow-100 text-yellow-600',
			'purple': 'bg-purple-100 text-purple-600',
			'pink': 'bg-pink-100 text-pink-600',
			'indigo': 'bg-indigo-100 text-indigo-600',
		};
		return colors[color || 'blue'] || colors['blue'];
	}
</script>

<div class="bg-white rounded-lg shadow border border-gray-200 overflow-hidden">
	<!-- Component Header -->
	{#if component.title}
		<div class="px-4 py-3 border-b border-gray-200 bg-gray-50">
			<h3 class="text-sm font-medium text-gray-900">{component.title}</h3>
		</div>
	{/if}

	<!-- Component Content -->
	<div class="p-4">
		{#if component.type === 'iframe' && isIframeConfig(component.config)}
			{@const config = component.config as IframeConfig}
			<iframe
				src={config.url}
				title={component.title || 'Embedded content'}
				class="w-full border-0 rounded"
				style="height: {config.height || 400}px;"
				sandbox={config.sandbox || 'allow-scripts allow-same-origin allow-forms allow-popups'}
				loading="lazy"
			></iframe>

		{:else if component.type === 'text' && isTextConfig(component.config)}
			{@const config = component.config as TextConfig}
			<div class="prose prose-sm max-w-none text-gray-700 whitespace-pre-wrap">
				{config.content}
			</div>

		{:else if component.type === 'markdown' && isTextConfig(component.config)}
			{@const config = component.config as TextConfig}
			<div class="prose prose-sm max-w-none">
				{@html renderMarkdown(config.content)}
			</div>

		{:else if component.type === 'html' && isHTMLConfig(component.config)}
			{@const config = component.config as HTMLConfig}
			<div class="custom-html">
				{@html config.content}
			</div>

		{:else if component.type === 'entity_list' && isEntityListConfig(component.config)}
			{@const config = component.config as EntityListConfig}
			<div class="text-center py-8 text-gray-500">
				<svg class="mx-auto h-8 w-8 text-gray-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
				</svg>
				<p class="text-sm">Entity List: <span class="font-medium">{config.entity}</span></p>
				<p class="text-xs text-gray-400 mt-1">Showing {config.pageSize || 10} records</p>
				<a href="/{config.entity}" class="mt-2 inline-block text-primary hover:text-blue-800 text-sm">
					View All
				</a>
			</div>

		{:else if component.type === 'link_group' && isLinkGroupConfig(component.config)}
			{@const config = component.config as LinkGroupConfig}
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
				{#each config.links as link}
					<a
						href={link.href}
						target={link.external ? '_blank' : undefined}
						rel={link.external ? 'noopener noreferrer' : undefined}
						class="flex items-center gap-3 p-3 rounded-lg border border-gray-200 hover:border-blue-300 hover:bg-blue-50 transition-colors"
					>
						<div class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center">
							<svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconPath(link.icon || 'link')} />
							</svg>
						</div>
						<div class="flex-1 min-w-0">
							<p class="text-sm font-medium text-gray-900 truncate">{link.label}</p>
							{#if link.description}
								<p class="text-xs text-gray-500 truncate">{link.description}</p>
							{/if}
						</div>
						{#if link.external}
							<svg class="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconPath('external-link')} />
							</svg>
						{/if}
					</a>
				{/each}
			</div>

		{:else if component.type === 'stats' && isStatsConfig(component.config)}
			{@const config = component.config as StatsConfig}
			<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
				{#each config.items as stat}
					<div class="text-center">
						<div class="inline-flex items-center justify-center w-12 h-12 rounded-full {getStatColorClass(stat.color)} mb-2">
							<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIconPath(stat.icon || 'chart')} />
							</svg>
						</div>
						<p class="text-2xl font-bold text-gray-900">{stat.value}</p>
						<p class="text-xs text-gray-500">{stat.label}</p>
					</div>
				{/each}
			</div>

		{:else}
			<div class="text-center py-8 text-gray-500">
				<p class="text-sm">Unknown component type: {component.type}</p>
			</div>
		{/if}
	</div>
</div>

<style>
	/* Sanitize HTML content */
	.custom-html :global(script),
	.custom-html :global(style),
	.custom-html :global(link) {
		display: none !important;
	}
</style>
