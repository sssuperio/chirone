<script lang="ts">
	import '../app.postcss';
	import { onMount } from 'svelte';

	import '$lib/app.css';
	import { base } from '$app/paths';

	import { collabStatus, initCollabSync } from '$lib/collab/client';
	import NavLink from '$lib/ui/navLink.svelte';

	const links = [
		{ href: `${base}/glyphs`, text: 'Glifi' },
		{ href: `${base}/syntax`, text: 'Sintassi' },
		{ href: `${base}/metrics`, text: 'Metriche' },
		{ href: `${base}/output`, text: 'Output' },
		{ href: `${base}/settings`, text: 'Impostazioni' }
	];

	onMount(() => {
		const stop = initCollabSync();
		return () => {
			stop();
		};
	});
</script>

<!--  -->
<div class="h-screen w-screen flex flex-col items-stretch overflow-hidden">
	<nav class="flex flex-row flex-nowrap py-2 px-4 bg-slate-900 space-x-2 shrink-0 items-center">
		{#each links as l}
			<NavLink href={l.href}>{l.text}</NavLink>
		{/each}

		<div class="ml-auto text-xs font-mono">
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
