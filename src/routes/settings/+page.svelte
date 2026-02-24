<script lang="ts">
	import { tick } from 'svelte';
	import { syntaxes, metrics, glyphs, selectedGlyph, defaultMetrics } from '$lib/stores';
	import { collabStatus } from '$lib/collab/client';
	import { normalizeFontMetrics } from '$lib/GTL/metrics';
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
					metrics: $metrics
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

	// Clear confirmation modal
	let clearModalOpen = false;
	let clearProjectInput = '';
	let clearProjectInputEl: HTMLInputElement | undefined;

	$: requiredProjectName = $collabStatus.project || 'default';
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
</script>

<!--  -->

<div class="p-8 space-y-8 flex flex-col">
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
