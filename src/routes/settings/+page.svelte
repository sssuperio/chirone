<script lang="ts">
	import { tick } from 'svelte';
	import {
		syntaxes,
		metrics,
		glyphs,
		selectedGlyph,
		defaultMetrics,
		fontMetadata,
		defaultMetadata
	} from '$lib/stores';
	import { collabStatus, setCollabProject } from '$lib/collab/client';
	import { normalizeFontMetrics } from '$lib/GTL/metrics';
	import { normalizeFontMetadata } from '$lib/GTL/metadata';
	import Upload from '$lib/ui/upload.svelte';
	import Button from '$lib/ui/button.svelte';
	import Tooltip from '$lib/ui/tooltip.svelte';
	import { Modal } from 'flowbite-svelte';
	import { currentSyntaxId } from '../syntax/+page.svelte';

	/**
	 * Clear project
	 */

	function clear() {
		exportFont();
		$syntaxes = [];
		$glyphs = [];
		$metrics = defaultMetrics;
		$fontMetadata = defaultMetadata;
		$selectedGlyph = '';
		$currentSyntaxId = '';
	}

	/**
	 * Export
	 */

	function exportFont() {
		let dataStr =
			'data:text/json;charset=utf-8,' +
			encodeURIComponent(
				JSON.stringify({
					glyphs: $glyphs,
					syntaxes: $syntaxes,
					metrics: $metrics,
					metadata: $fontMetadata
				})
			);
		let dlAnchorElem = document.createElement('a');
		dlAnchorElem.setAttribute('href', dataStr);
		dlAnchorElem.setAttribute('download', 'GTL.json');
		dlAnchorElem.click();
	}

	/**
	 * Import font
	 */

	function importFont(e: any) {
		try {
			//
			const json = (e as any).detail.json;
			const res = JSON.parse(json);
			//
			$syntaxes = res['syntaxes'];
			$glyphs = res['glyphs'];
			$metrics = normalizeFontMetrics(res['metrics'] || defaultMetrics);
			$fontMetadata = normalizeFontMetadata(res['metadata'] || defaultMetadata);
			//
			$selectedGlyph = '';
			$currentSyntaxId = '';
			//
			importTooltipOk = true;
		} catch (e) {
			importTooltipFail = true;
			console.log('ImportError');
		}
	}

	// Tooltip import
	let importTooltipOk = false;
	let importTooltipFail = false;

	// Collab project switch
	const projectNamePattern = /^[a-zA-Z0-9_-]+$/;
	let projectNameInput = '';
	let previousProjectFromStatus = '';

	// Clear confirmation modal
	let clearModalOpen = false;
	let clearProjectInput = '';
	let clearProjectInputEl: HTMLInputElement | undefined;

	$: requiredProjectName = $collabStatus.project || 'default';
	$: {
		const currentProject = $collabStatus.project || 'default';
		if (!projectNameInput || projectNameInput === previousProjectFromStatus) {
			projectNameInput = currentProject;
		}
		previousProjectFromStatus = currentProject;
	}
	$: sanitizedProjectNameInput = projectNameInput.trim();
	$: isProjectNameValid = projectNamePattern.test(sanitizedProjectNameInput);
	$: canApplyProjectName = isProjectNameValid && sanitizedProjectNameInput !== requiredProjectName;
	$: canConfirmClear = clearProjectInput === requiredProjectName;

	async function openClearModal() {
		clearProjectInput = '';
		clearModalOpen = true;
		await tick();
		clearProjectInputEl?.focus();
	}

	function closeClearModal() {
		clearModalOpen = false;
		clearProjectInput = '';
	}

	function confirmClearProject() {
		if (!canConfirmClear) return;
		clear();
		closeClearModal();
	}

	function closeTooltipFail() {
		importTooltipFail = false;
	}

	function applyProjectName() {
		if (!canApplyProjectName) return;
		projectNameInput = setCollabProject(sanitizedProjectNameInput);
	}
</script>

<!--  -->

<div class="p-8 space-y-8 flex flex-col">
	<div class="space-y-2">
		<p class="font-mono">Nome progetto collab</p>
		<p class="font-mono text-sm text-slate-600">
			Usa solo lettere, numeri, `_` e `-`. Cambiando progetto, la sync passa subito al nuovo namespace.
		</p>
		<div class="flex gap-2 items-center">
			<input
				class="w-full max-w-md border border-slate-400 px-3 py-2"
				placeholder="default"
				bind:value={projectNameInput}
			/>
			<Button disabled={!canApplyProjectName} on:click={applyProjectName}>Applica progetto</Button>
		</div>
		{#if !isProjectNameValid}
			<p class="font-mono text-xs text-rose-700">Nome non valido. Caratteri ammessi: `a-z A-Z 0-9 _ -`.</p>
		{/if}
	</div>

	<div class="space-y-3">
		<p class="font-mono">Metadata font (download OTF)</p>
		<p class="font-mono text-sm text-slate-600">
			Questi valori vengono precompilati nel file scaricato, ma puoi modificarli quando vuoi.
		</p>
		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaName">Nome</label>
				<input
					id="fontMetaName"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="Nome font"
					bind:value={$fontMetadata.name}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaFamily">Nome famiglia</label>
				<input
					id="fontMetaFamily"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="GTL"
					bind:value={$fontMetadata.familyName}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaVersion">Versione</label>
				<input
					id="fontMetaVersion"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="Version 1.0"
					bind:value={$fontMetadata.version}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaCreated">Data di creazione</label>
				<input
					id="fontMetaCreated"
					type="date"
					class="w-full border border-slate-400 px-3 py-2"
					bind:value={$fontMetadata.createdDate}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaDesigner">Progettisti</label>
				<input
					id="fontMetaDesigner"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="Nome progettista"
					bind:value={$fontMetadata.designer}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaManufacturer">Produttore</label>
				<input
					id="fontMetaManufacturer"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="Nome produttore"
					bind:value={$fontMetadata.manufacturer}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaDesignerURL">URL progettista</label>
				<input
					id="fontMetaDesignerURL"
					type="url"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="https://"
					bind:value={$fontMetadata.designerURL}
				/>
			</div>
			<div class="space-y-1">
				<label class="font-mono text-sm" for="fontMetaManufacturerURL">URL produttore</label>
				<input
					id="fontMetaManufacturerURL"
					type="url"
					class="w-full border border-slate-400 px-3 py-2"
					placeholder="https://"
					bind:value={$fontMetadata.manufacturerURL}
				/>
			</div>
		</div>
		<div class="space-y-1">
			<label class="font-mono text-sm" for="fontMetaLicense">Licenza</label>
			<textarea
				id="fontMetaLicense"
				class="w-full min-h-20 border border-slate-400 px-3 py-2"
				placeholder="Testo licenza"
				bind:value={$fontMetadata.license}
			/>
		</div>
	</div>

	<div>
		<p class=" font-mono">Elimina progetto</p>
		<p class="font-mono text-sm mb-2 text-slate-600">
			Il progetto verrà esportato automaticamente prima di essere eliminato
		</p>
		<button
			class="flex bg-red-700 text-white p-3 hover:bg-red-800 font-mono disabled:opacity-40 disabled:hover:bg-red-700"
			on:click={openClearModal}
		>
			Clear
		</button>
	</div>
	<div>
		<p class="mb-2 font-mono">Esporta progetto</p>
		<Button on:click={exportFont}>Esporta</Button>
	</div>
	<div class="space-y-2">
		<p class="font-mono">Importa progetto</p>
		<Upload on:upload={importFont}>Importa</Upload>

		<!-- Tooltips -->
		<Tooltip bind:visible={importTooltipOk} state="positive">
			<p class="font-mono text-slate-900">
				Caricamento riuscito! Controlla la sintassi e i glifi :)
			</p>
		</Tooltip>
		<Tooltip bind:visible={importTooltipFail} state="negative">
			<div class="flex flex-row justify-between items-center">
				<p class="font-mono text-slate-900">Errore di caricamento :(</p>
				<Button on:click={closeTooltipFail}>X</Button>
			</div>
		</Tooltip>
	</div>
</div>

<Modal outsideclose class="!rounded-none-none !font-mono" bind:open={clearModalOpen} title="Conferma eliminazione">
	<div class="space-y-3 font-mono">
		<p class="text-sm text-slate-800">
			Questa operazione elimina tutto il progetto corrente. Per confermare scrivi il nome progetto:
			<span class="font-bold">{requiredProjectName}</span>
		</p>

		<input
			class="w-full border border-slate-400 px-3 py-2"
			placeholder={requiredProjectName}
			bind:this={clearProjectInputEl}
			bind:value={clearProjectInput}
		/>

		<div class="flex gap-2">
			<Button on:click={closeClearModal}>Annulla</Button>
			<button
				class="flex bg-red-700 text-white p-3 hover:bg-red-800 font-mono disabled:opacity-40 disabled:hover:bg-red-700"
				disabled={!canConfirmClear}
				on:click={confirmClearProject}
			>
				Elimina progetto
			</button>
		</div>
	</div>
</Modal>
