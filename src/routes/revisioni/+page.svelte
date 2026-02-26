<script lang="ts">
	import { onMount } from 'svelte';
	import { collabStatus } from '$lib/collab/client';
	import Button from '$lib/ui/button.svelte';

	type RevisionMeta = {
		id: string;
		version: number;
		createdAt: string;
		message: string;
	};

	type RevisionsResponse = {
		project: string;
		currentVersion: number;
		suggestedMessage: string;
		revisions: Array<RevisionMeta>;
	};

	const collabServer = (import.meta.env.VITE_COLLAB_SERVER as string | undefined)?.trim() ?? '';
	const collabEnabled = collabServer.length > 0;
	const collabServerBase = collabServer.replace(/\/+$/g, '');

	let loading = false;
	let saving = false;
	let revertingRevisionID = '';
	let errorMessage = '';
	let successMessage = '';

	let loadedProject = '';
	let currentVersion = 0;
	let suggestedMessage = '';
	let messageDraft = '';
	let messageTouched = false;
	let revisions: Array<RevisionMeta> = [];

	function isObjectRecord(value: unknown): value is Record<string, unknown> {
		return typeof value === 'object' && value !== null;
	}

	function sanitizeProjectID(raw: string): string {
		return /^[a-zA-Z0-9_-]+$/.test(raw) ? raw : 'default';
	}

	function normalizeRevisionMeta(value: unknown): RevisionMeta | null {
		if (!isObjectRecord(value)) return null;
		if (typeof value.id !== 'string' || !value.id.trim()) return null;
		if (typeof value.version !== 'number' || !Number.isFinite(value.version)) return null;
		if (typeof value.createdAt !== 'string') return null;
		if (typeof value.message !== 'string') return null;
		return {
			id: value.id,
			version: Math.max(0, Math.trunc(value.version)),
			createdAt: value.createdAt,
			message: value.message
		};
	}

	function normalizeRevisionsResponse(value: unknown): RevisionsResponse | null {
		if (!isObjectRecord(value)) return null;
		if (typeof value.project !== 'string') return null;
		if (typeof value.currentVersion !== 'number' || !Number.isFinite(value.currentVersion)) return null;
		if (typeof value.suggestedMessage !== 'string') return null;
		if (!Array.isArray(value.revisions)) return null;

		const parsedRevisions: Array<RevisionMeta> = [];
		for (const item of value.revisions) {
			const parsed = normalizeRevisionMeta(item);
			if (!parsed) return null;
			parsedRevisions.push(parsed);
		}

		return {
			project: sanitizeProjectID(value.project),
			currentVersion: Math.max(0, Math.trunc(value.currentVersion)),
			suggestedMessage: value.suggestedMessage,
			revisions: parsedRevisions
		};
	}

	function revisionsURL(projectID: string): string {
		return `${collabServerBase}/api/revisions?project=${encodeURIComponent(projectID)}`;
	}

	function revisionRevertURL(projectID: string): string {
		return `${collabServerBase}/api/revisions/revert?project=${encodeURIComponent(projectID)}`;
	}

	function formatDate(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat('it-IT', {
			dateStyle: 'medium',
			timeStyle: 'medium'
		}).format(date);
	}

	async function loadRevisions(projectID: string, forcePrefillMessage = false) {
		if (!collabEnabled) return;
		loading = true;
		errorMessage = '';
		successMessage = '';

		try {
			const response = await fetch(revisionsURL(projectID), { cache: 'no-store' });
			if (!response.ok) {
				throw new Error(`load failed: ${response.status}`);
			}

			const payload = (await response.json()) as unknown;
			const parsed = normalizeRevisionsResponse(payload);
			if (!parsed) {
				throw new Error('invalid revisions payload');
			}

			loadedProject = parsed.project;
			currentVersion = parsed.currentVersion;
			suggestedMessage = parsed.suggestedMessage;
			revisions = parsed.revisions;
			if (forcePrefillMessage || !messageTouched || !messageDraft.trim()) {
				messageDraft = suggestedMessage;
				messageTouched = false;
			}
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'load failed';
		} finally {
			loading = false;
		}
	}

	async function saveRevision() {
		if (!collabEnabled || saving || loading) return;
		const projectID = sanitizeProjectID($collabStatus.project || 'default');
		saving = true;
		errorMessage = '';
		successMessage = '';

		try {
			const response = await fetch(revisionsURL(projectID), {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					message: messageDraft
				})
			});
			if (!response.ok) {
				throw new Error(`save failed: ${response.status}`);
			}

			messageTouched = false;
			await loadRevisions(projectID, true);
			successMessage = 'Revisione salvata';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'save failed';
		} finally {
			saving = false;
		}
	}

	async function restoreRevision(revision: RevisionMeta) {
		if (!collabEnabled || revertingRevisionID || saving) return;
		const projectID = sanitizeProjectID($collabStatus.project || 'default');
		const shouldContinue = window.confirm(
			`Ripristinare la revisione ${revision.id}? Il progetto corrente verrà sostituito.`
		);
		if (!shouldContinue) return;

		revertingRevisionID = revision.id;
		errorMessage = '';
		successMessage = '';

		try {
			const response = await fetch(revisionRevertURL(projectID), {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					id: revision.id
				})
			});
			if (!response.ok) {
				throw new Error(`revert failed: ${response.status}`);
			}

			messageTouched = false;
			await loadRevisions(projectID, true);
			successMessage = `Ripristinata revisione ${revision.id}`;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'revert failed';
		} finally {
			revertingRevisionID = '';
		}
	}

	function useSuggestedMessage() {
		messageDraft = suggestedMessage;
		messageTouched = false;
	}

	function handleMessageInput() {
		messageTouched = true;
	}

	$: activeProject = sanitizeProjectID($collabStatus.project || 'default');
	$: collabState = $collabStatus.state;
	$: if (
		collabEnabled &&
		activeProject &&
		activeProject !== loadedProject &&
		collabState !== 'disabled'
	) {
		messageTouched = false;
		void loadRevisions(activeProject, true);
	}

	onMount(() => {
		if (collabEnabled) {
			void loadRevisions(activeProject, true);
		}
	});
</script>

<svelte:head>
	<title>Revisioni</title>
</svelte:head>

<div class="h-full overflow-y-auto p-8 space-y-6">
	{#if !collabEnabled}
		<p class="font-mono text-sm text-slate-600">Collab non configurato (`VITE_COLLAB_SERVER` mancante).</p>
	{:else}
		<div class="space-y-2">
			<h1 class="font-mono text-xl">Revisioni</h1>
			<p class="font-mono text-sm text-slate-600">
				Progetto: <span class="font-semibold text-slate-900">{activeProject}</span> · Versione corrente:
				<span class="font-semibold text-slate-900">{currentVersion}</span>
			</p>
			<div class="flex flex-wrap gap-2">
				<Button on:click={() => loadRevisions(activeProject, false)} disabled={loading || saving}
					>Aggiorna</Button
				>
			</div>
		</div>

		<div class="space-y-3 border border-slate-300 bg-white p-4">
			<p class="font-mono text-sm text-slate-700">Messaggio revisione (prefill automatico dai cambiamenti)</p>
			<textarea
				class="w-full min-h-28 border border-slate-300 p-3 font-mono text-sm"
				bind:value={messageDraft}
				on:input={handleMessageInput}
			/>
			<div class="flex flex-wrap gap-2">
				<Button on:click={saveRevision} disabled={saving || loading}>Salva revisione</Button>
				<Button on:click={useSuggestedMessage} disabled={saving || loading}>Usa suggerito</Button>
			</div>
			<p class="font-mono text-xs text-slate-500">Suggerito: {suggestedMessage}</p>
		</div>

		{#if errorMessage}
			<p class="font-mono text-sm text-rose-700">{errorMessage}</p>
		{/if}
		{#if successMessage}
			<p class="font-mono text-sm text-emerald-700">{successMessage}</p>
		{/if}

		<div class="space-y-2">
			<h2 class="font-mono text-lg">Storico</h2>
			{#if loading && revisions.length === 0}
				<p class="font-mono text-sm text-slate-600">Caricamento revisioni...</p>
			{:else if revisions.length === 0}
				<p class="font-mono text-sm text-slate-600">Nessuna revisione salvata.</p>
			{:else}
				<div class="space-y-3">
					{#each revisions as revision (revision.id)}
						<div class="border border-slate-300 bg-white p-4 space-y-2">
							<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
								<p class="font-mono text-sm text-slate-900 font-semibold">{revision.message}</p>
								<span class="font-mono text-xs text-slate-500">id: {revision.id}</span>
							</div>
							<p class="font-mono text-xs text-slate-600">
								Versione progetto: {revision.version} · Salvata: {formatDate(revision.createdAt)}
							</p>
							<div class="flex flex-wrap gap-2">
								<Button
									on:click={() => restoreRevision(revision)}
									disabled={Boolean(revertingRevisionID) || saving}
								>
									{#if revertingRevisionID === revision.id}
										Ripristino...
									{:else}
										Ripristina qui
									{/if}
								</Button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
