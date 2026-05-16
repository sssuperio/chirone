<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { setCollabProject, collabConfig, collabStatus } from '$lib/collab/client';
	import Button from '$lib/ui/button.svelte';

	const dispatch = createEventDispatcher();

	interface ProjectMeta {
		project: string;
		version: number;
		updatedAt: string;
		hasPassword: boolean;
	}

	let projects: ProjectMeta[] = [];
	let loading = true;
	let error = '';
	let selectedProject = '';
	let password = '';
	let passwordError = '';
	let authenticating = false;

	$: projectsBase = $collabConfig.base;

	async function loadProjects() {
		loading = true;
		error = '';
		try {
			const url = `${projectsBase}/api/projects`;
			const res = await fetch(url, { cache: 'no-store' });
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const data = await res.json();
			projects = data.projects ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	async function handleAuth() {
		if (!selectedProject) return;
		authenticating = true;
		passwordError = '';
		try {
			const url = `${projectsBase}/api/project/auth?project=${encodeURIComponent(selectedProject)}`;
			const res = await fetch(url, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ password })
			});
			if (!res.ok) {
				passwordError = 'Password non valida';
				return;
			}
			// Store password in localStorage for subsequent requests
			localStorage.setItem(`chirone-pwd-${selectedProject}`, password);
			selectProject(selectedProject);
		} catch (e) {
			passwordError = e instanceof Error ? e.message : 'Errore';
		} finally {
			authenticating = false;
		}
	}

	function selectProject(id: string) {
		const project = projects.find((p) => p.project === id);
		if (!project) return;

		if (project.hasPassword) {
			const saved = localStorage.getItem(`chirone-pwd-${id}`);
			if (saved) {
				password = saved;
			} else {
				selectedProject = id;
				password = '';
				return;
			}
		}

		setCollabProject(id);
		dispatch('select', { project: id });
	}

	onMount(() => {
		loadProjects();
	});
</script>

<div class="flex h-full w-full items-center justify-center bg-slate-100">
	<div class="w-full max-w-md space-y-6 rounded bg-white p-8 shadow-lg">
		<h1 class="font-mono text-xl">Chirone — Progetti</h1>

		{#if loading}
			<p class="font-mono text-sm text-slate-500">Caricamento progetti...</p>
		{:else if error}
			<p class="font-mono text-sm text-red-600">{error}</p>
			<Button on:click={loadProjects}>Riprova</Button>
		{:else if selectedProject && projects.find((p) => p.project === selectedProject)?.hasPassword && !localStorage.getItem(`chirone-pwd-${selectedProject}`)}
			<div class="space-y-3 font-mono">
				<p class="text-sm text-slate-600">
					Il progetto <span class="font-bold">{selectedProject}</span> è protetto da password.
				</p>
				<input
					type="password"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="Password"
					bind:value={password}
					on:keydown={(e) => e.key === 'Enter' && handleAuth()}
				/>
				{#if passwordError}
					<p class="text-xs text-red-600">{passwordError}</p>
				{/if}
				<div class="flex gap-2">
					<Button on:click={() => (selectedProject = '')}>Indietro</Button>
					<Button disabled={!password || authenticating} on:click={handleAuth}>
						{authenticating ? 'Verifica...' : 'Accedi'}
					</Button>
				</div>
			</div>
		{:else if projects.length === 0}
			<p class="font-mono text-sm text-slate-500">
				Nessun progetto trovato sul server. Il progetto "default" verrà creato automaticamente.
			</p>
			<Button on:click={() => selectProject('default')}>Usa progetto "default"</Button>
		{:else}
			<div class="space-y-2">
				<p class="font-mono text-xs text-slate-500">Seleziona un progetto dal server:</p>
				<div class="max-h-64 space-y-1 overflow-y-auto">
					{#each projects as p (p.project)}
						<button
							class="flex w-full items-center justify-between rounded px-3 py-2 text-left font-mono text-sm hover:bg-slate-100"
							on:click={() => selectProject(p.project)}
						>
							<span>{p.project}</span>
							<span class="text-xs text-slate-400">
								{#if p.hasPassword}🔒{/if}
								v{p.version}
							</span>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<div class="font-mono text-xs text-slate-400">
			Server: {$collabConfig.server || 'nessuno'}
			{#if $collabStatus.project}
				&middot; Progetto: {$collabStatus.project}{/if}
		</div>
	</div>
</div>
