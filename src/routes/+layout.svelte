<script lang="ts">
	import '../app.postcss';
	import { onMount } from 'svelte';

	import '$lib/app.css';
	import { base } from '$app/paths';

	import { collabStatus, initCollabSync } from '$lib/collab/client';
	import { themeMode } from '$lib/stores';
	import NavLink from '$lib/ui/navLink.svelte';

	const links = [
		{ href: `${base}/glyphs`, text: 'Glifi' },
		{ href: `${base}/syntax`, text: 'Sintassi' },
		{ href: `${base}/metrics`, text: 'Metriche' },
		{ href: `${base}/output`, text: 'Output' },
		{ href: `${base}/settings`, text: 'Impostazioni' }
	];

	onMount(() => {
		document.documentElement.classList.remove('theme-dark');
		const stop = initCollabSync();
		return () => {
			stop();
		};
	});

	$: isDarkMode = $themeMode === 'dark';

	function toggleThemeMode() {
		themeMode.set(isDarkMode ? 'light' : 'dark');
	}
</script>

<!--  -->
<div class="h-screen w-screen flex flex-col items-stretch overflow-hidden">
	<nav class="flex flex-row flex-nowrap py-2 px-4 bg-slate-900 space-x-2 shrink-0 items-center">
		{#each links as l}
			<NavLink href={l.href}>{l.text}</NavLink>
		{/each}

		<div class="ml-auto flex items-center gap-3 text-xs font-mono">
			<button
				type="button"
				on:click={toggleThemeMode}
				class="px-2 py-1 border border-slate-500 text-slate-200 hover:bg-slate-700"
				aria-label="Toggle night mode"
				title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}
			>
				{isDarkMode ? 'Light' : 'Night'}
			</button>

			{#if $collabStatus.enabled}
				<span
					class:text-emerald-300={$collabStatus.state === 'connected'}
					class:text-amber-300={$collabStatus.state === 'connecting'}
					class:text-rose-300={$collabStatus.state === 'offline' || $collabStatus.state === 'error'}
					class:text-slate-400={$collabStatus.state === 'disabled'}
				>
					Collab: {$collabStatus.state}
				</span>
				<span class="text-slate-400 ml-2">{$collabStatus.project}</span>
			{:else}
				<span class="text-slate-500">Collab off</span>
			{/if}
		</div>
	</nav>

	<main class="h-0 grow overflow-hidden flex flex-row items-stretch font-mono">
		<slot />
	</main>
</div>
