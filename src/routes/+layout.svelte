<script lang="ts">
	import '../app.postcss';
	import { onMount } from 'svelte';

	import '$lib/app.css';
	import { base } from '$app/paths';

	import {
		appVersion,
		collabServerSHA,
		collabStatus,
		initAppVersionInfo,
		initCollabSync
	} from '$lib/collab/client';
	import { migrateFromLegacyStores } from '$lib/stores/migration';
	import { activeFontId, fontDefinitions } from '$lib/stores';
	import NavLink from '$lib/ui/navLink.svelte';

	const links = [
		{ href: `${base}/glyphs`, text: 'Glifi' },
		{ href: `${base}/syntax`, text: 'Sintassi' },
		{ href: `${base}/metrics`, text: 'Metriche' },
		{ href: `${base}/output`, text: 'Output' },
		{ href: `${base}/revisioni`, text: 'Revisioni' },
		{ href: `${base}/project`, text: 'Progetto' }
	];

	onMount(() => {
		migrateFromLegacyStores();
		initAppVersionInfo();
		const stop = initCollabSync();
		return () => {
			stop();
		};
	});
</script>

<!--  -->
<div class="flex h-screen w-screen flex-col items-stretch overflow-hidden">
	<nav class="flex shrink-0 flex-row flex-nowrap items-center space-x-2 bg-slate-900 px-4 py-2">
		{#each links as l}
			<NavLink href={l.href}>{l.text}</NavLink>
		{/each}

		<div class="ml-auto flex min-w-0 items-center gap-3 font-mono text-xs">
			{#if $fontDefinitions.length > 0}
				<select
					class="border border-slate-600 bg-slate-800 px-2 py-1 text-xs text-slate-200"
					bind:value={$activeFontId}
				>
					{#each $fontDefinitions as f (f.id)}
						<option value={f.id}>{f.name}</option>
					{/each}
				</select>
			{/if}
		</div>

		<div class="flex min-w-0 items-center gap-3 font-mono text-xs">
			{#if $collabStatus.enabled}
				<span
					class:text-emerald-300={$collabStatus.state === 'connected'}
					class:text-amber-300={$collabStatus.state === 'connecting'}
					class:text-rose-300={$collabStatus.state === 'offline' || $collabStatus.state === 'error'}
					class:text-slate-400={$collabStatus.state === 'disabled'}
				>
					Collab: {$collabStatus.state}
				</span>
				<div class="flex min-w-0 items-center gap-3 text-slate-400">
					<span class="truncate font-semibold text-slate-200">{$collabStatus.project}</span>
					<span>v{$appVersion}</span>
					<span>SHA {$collabServerSHA}</span>
				</div>
			{:else}
				<span class="text-slate-500">Collab off</span>
			{/if}
		</div>
	</nav>

	<main class="flex h-0 grow flex-row items-stretch overflow-hidden font-mono">
		<slot />
	</main>
</div>
