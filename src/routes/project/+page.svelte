<script lang="ts">
	import { nanoid } from 'nanoid';
	import {
		projectInfo,
		fontDefinitions,
		metricsPresets,
		metadataPresets,
		syntaxes,
		glyphs,
		metrics,
		fontMetadata,
		selectedGlyph,
		resolvePresetName,
		resolveSyntaxName,
		activeFontId,
		loadGlyphsForFont,
		saveGlyphsForFont
	} from '$lib/stores';
	import { normalizeFontMetadata, defaultFontMetadata } from '$lib/GTL/metadata';
	import {
		createProjectArchive,
		parseProjectArchive,
		parseLegacyProjectFile,
		detectArchiveFormat
	} from '$lib/archive';
	import type { MetadataPreset, FontDefinition } from '$lib/types';
	import Button from '$lib/ui/button.svelte';
	import Upload from '$lib/ui/upload.svelte';
	import { Modal } from 'flowbite-svelte';

	let editingFont: FontDefinition | null = null;
	let fontFormOpen = false;
	let deleteFontId: string | null = null;
	$: deleteModalOpen = deleteFontId !== null;

	// Font form fields
	let fontName = '';
	let fontSyntaxId = '';
	let fontMetricsId = '';
	let fontMetadataId = '';
	let fontOutputName = '';
	let fontEnabled = true;

	// Project name editing
	let projectNameInput = $projectInfo.name;

	// Metadata editor for active preset
	let editingMetadata = false;
	let metadataForm = { ...$fontMetadata };

	function openNewFont() {
		editingFont = null;
		fontName = 'Regular';
		fontSyntaxId = $syntaxes[0]?.id ?? '';
		fontMetricsId = $metricsPresets[0]?.id ?? '';
		fontMetadataId = $metadataPresets[0]?.id ?? '';
		fontOutputName = '';
		fontEnabled = true;
		fontFormOpen = true;
	}

	function openEditFont(font: FontDefinition) {
		editingFont = font;
		fontName = font.name;
		fontSyntaxId = font.syntaxId;
		fontMetricsId = font.metricsId;
		fontMetadataId = font.metadataId;
		fontOutputName = font.outputName;
		fontEnabled = font.enabled;
		fontFormOpen = true;
	}

	function closeFontForm() {
		fontFormOpen = false;
		editingFont = null;
	}

	function deriveOutputName(): string {
		const meta = $metadataPresets.find((m) => m.id === fontMetadataId);
		const family = meta?.familyName || 'GTL';
		return `${family}-${fontName || 'Regular'}.otf`;
	}

	function saveFont() {
		const outputName = fontOutputName || deriveOutputName();
		const def: FontDefinition = {
			id: editingFont?.id ?? nanoid(),
			name: fontName || 'Regular',
			syntaxId: fontSyntaxId,
			metricsId: fontMetricsId,
			metadataId: fontMetadataId,
			outputName,
			enabled: fontEnabled
		};

		const current = $fontDefinitions;
		if (editingFont) {
			const idx = current.findIndex((f) => f.id === editingFont!.id);
			if (idx >= 0) {
				current[idx] = def;
				fontDefinitions.set([...current]);
			}
		} else {
			fontDefinitions.set([...current, def]);
		}

		closeFontForm();
	}

	function confirmDeleteFont(id: string) {
		fontDefinitions.set($fontDefinitions.filter((f) => f.id !== id));
		deleteFontId = null;
	}

	function duplicateFont(font: FontDefinition) {
		const copy: FontDefinition = {
			...font,
			id: nanoid(),
			name: `${font.name} (copy)`,
			outputName: font.outputName.replace(/\.otf$/, ' (copy).otf')
		};
		fontDefinitions.set([...$fontDefinitions, copy]);

		// Copy the source font's glyphs to the new font
		const sourceGlyphs = font.id === $activeFontId ? $glyphs : loadGlyphsForFont(font.id);
		if (sourceGlyphs.length > 0) {
			saveGlyphsForFont(copy.id, JSON.parse(JSON.stringify(sourceGlyphs)));
		}
	}

	function saveProjectName() {
		const trimmed = projectNameInput.trim();
		if (trimmed && trimmed !== $projectInfo.name) {
			projectInfo.set({
				...$projectInfo,
				name: trimmed,
				updatedAt: new Date().toISOString()
			});
		}
	}

	function openMetadataEditor() {
		const active = $metadataPresets[0];
		if (active) {
			metadataForm = { ...active };
		} else {
			metadataForm = { ...defaultFontMetadata };
		}
		editingMetadata = true;
	}

	function saveMetadata() {
		const normalized = normalizeFontMetadata(metadataForm);
		fontMetadata.set(normalized);

		const current = $metadataPresets;
		const preset: MetadataPreset = {
			...normalized,
			id: current[0]?.id ?? 'default',
			name: current[0]?.name ?? 'Default'
		};
		if (current.length === 0) {
			metadataPresets.set([preset]);
		} else {
			current[0] = preset;
			metadataPresets.set([...current]);
		}
		editingMetadata = false;
	}

	async function exportProjectArchive() {
		try {
			const blob = await createProjectArchive(
				$glyphs,
				$syntaxes,
				$metricsPresets,
				$metadataPresets,
				$fontDefinitions,
				$projectInfo
			);
			const url = URL.createObjectURL(blob);
			const name = ($projectInfo.name || 'project').replace(/\s+/g, '-');
			const link = document.createElement('a');
			link.href = url;
			link.download = `${name}.chirone.tar.gz`;
			document.body.appendChild(link);
			link.click();
			link.remove();
			setTimeout(() => URL.revokeObjectURL(url), 0);
		} catch (e) {
			console.error('Archive export failed', e);
			alert('Esportazione fallita: ' + (e instanceof Error ? e.message : String(e)));
		}
	}

	async function handleArchiveFile(file: File) {
		const format = detectArchiveFormat(file);
		let parsed: Awaited<ReturnType<typeof parseProjectArchive>>;
		let summary: string;

		if (format === 'chirone') {
			parsed = await parseProjectArchive(file);
			summary = [
				`Progetto: ${parsed.projectInfo.name}`,
				`Glifi: ${parsed.glyphs.length}`,
				`Sintassi: ${parsed.syntaxes.length}`,
				`Metrics presets: ${parsed.metricsPresets.length}`,
				`Metadata presets: ${parsed.metadataPresets.length}`,
				`Font: ${parsed.fontDefinitions.length}`
			].join('\n');
		} else {
			const text = await file.text();
			parsed = parseLegacyProjectFile(text);
			summary = [
				`Formato legacy GTL.json`,
				`Glifi: ${parsed.glyphs.length}`,
				`Sintassi: ${parsed.syntaxes.length}`,
				`Creato 1 metrics preset "Default"`,
				`Creato 1 metadata preset "Default"`,
				`Creato 1 font "Regular"`
			].join('\n');
		}

		importSummary = summary;
		importData = parsed;
		importModalOpen = true;
	}

	function confirmImport() {
		if (!importData) return;

		// Auto-backup current state before replacing
		exportProjectArchive().catch(() => {});

		projectInfo.set(importData.projectInfo);
		glyphs.set(importData.glyphs);
		syntaxes.set(importData.syntaxes);
		metricsPresets.set(importData.metricsPresets);
		metadataPresets.set(importData.metadataPresets);
		fontDefinitions.set(importData.fontDefinitions);

		if (importData.metricsPresets.length > 0) {
			metrics.set(importData.metricsPresets[0]);
		}
		if (importData.metadataPresets.length > 0) {
			fontMetadata.set(importData.metadataPresets[0]);
		}
		selectedGlyph.set('');

		importModalOpen = false;
		importData = null;
		importSummary = '';
	}

	let importModalOpen = false;
	let importSummary = '';
	let importData: Awaited<ReturnType<typeof parseProjectArchive>> | null = null;

	function handleUpload(event: CustomEvent<{ json: string }>) {
		// Legacy JSON via upload component
		const parsed = parseLegacyProjectFile(event.detail.json);
		importSummary = [
			'Formato legacy GTL.json',
			`Glifi: ${parsed.glyphs.length}`,
			`Sintassi: ${parsed.syntaxes.length}`
		].join('\n');
		importData = parsed;
		importModalOpen = true;
	}

	async function handleFileChange(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		await handleArchiveFile(file);
		input.value = '';
	}

	// Validation
	$: fontSyntaxValid = fontSyntaxId && $syntaxes.some((s) => s.id === fontSyntaxId);
	$: fontMetricsValid = fontMetricsId && $metricsPresets.some((m) => m.id === fontMetricsId);
	$: fontMetadataValid = fontMetadataId && $metadataPresets.some((m) => m.id === fontMetadataId);
	$: fontFormValid = fontName.trim() && fontSyntaxValid && fontMetricsValid && fontMetadataValid;

	// Reference warnings per font
	$: fontWarnings = new Map(
		$fontDefinitions.map((f) => {
			const missing: string[] = [];
			if (f.syntaxId && !$syntaxes.some((s) => s.id === f.syntaxId)) missing.push('syntax');
			if (f.metricsId && !$metricsPresets.some((m) => m.id === f.metricsId))
				missing.push('metrics');
			if (f.metadataId && !$metadataPresets.some((m) => m.id === f.metadataId))
				missing.push('metadata');
			return [f.id, missing];
		})
	);
</script>

<div class="flex grow flex-col overflow-y-auto overflow-x-hidden">
	<div class="space-y-8 p-8">
		<!-- Project Name -->
		<div class="space-y-2">
			<p class="font-mono text-lg">Progetto</p>
			<div class="flex items-center gap-2">
				<input
					class="w-full max-w-md border border-slate-400 px-3 py-2 font-mono"
					bind:value={projectNameInput}
					on:blur={saveProjectName}
				/>
				<Button on:click={saveProjectName}>Salva nome</Button>
			</div>
			<div class="flex gap-4 font-mono text-xs text-slate-400">
				<span>ID: {$projectInfo.id}</span>
				<span>Creato: {new Date($projectInfo.createdAt).toLocaleDateString()}</span>
				<span>Aggiornato: {new Date($projectInfo.updatedAt).toLocaleDateString()}</span>
			</div>
		</div>

		<!-- Font Definitions -->
		<div class="space-y-3">
			<div class="flex items-center gap-4">
				<p class="font-mono text-lg">Font</p>
				<Button on:click={openNewFont}>+ Nuovo font</Button>
			</div>

			{#if $fontDefinitions.length === 0}
				<p class="font-mono text-sm text-slate-500">
					Nessun font definito. Crea un font per iniziare.
				</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full border-collapse font-mono text-sm">
						<thead>
							<tr class="border-b border-slate-300 text-left">
								<th class="px-3 py-2">Nome</th>
								<th class="px-3 py-2">Syntax</th>
								<th class="px-3 py-2">Metriche</th>
								<th class="px-3 py-2">Metadata</th>
								<th class="px-3 py-2">Output Name</th>
								<th class="px-3 py-2">Stato</th>
								<th class="px-3 py-2">Azioni</th>
							</tr>
						</thead>
						<tbody>
							{#each $fontDefinitions as font (font.id)}
								{@const warnings = fontWarnings.get(font.id) ?? []}
								<tr class="border-b border-slate-200 hover:bg-slate-50">
									<td class="px-3 py-2">{font.name}</td>
									<td class="px-3 py-2 text-slate-600">
										{resolveSyntaxName($syntaxes, font.syntaxId)}
									</td>
									<td class="px-3 py-2 text-slate-600">
										{resolvePresetName($metricsPresets, font.metricsId)}
									</td>
									<td class="px-3 py-2 text-slate-600">
										{resolvePresetName($metadataPresets, font.metadataId)}
									</td>
									<td class="px-3 py-2 text-slate-500">{font.outputName}</td>
									<td class="px-3 py-2">
										{#if font.enabled}
											<span class="text-emerald-600">Attivo</span>
										{:else}
											<span class="text-slate-400">Disattivo</span>
										{/if}
										{#if warnings.length > 0}
											<span class="ml-2 text-amber-600" title={warnings.join(', ') + ' mancante'}>
												⚠
											</span>
										{/if}
									</td>
									<td class="flex gap-1 px-3 py-2">
										<button
											class="px-2 py-1 text-xs hover:bg-slate-200"
											on:click={() => openEditFont(font)}>Modifica</button
										>
										<button
											class="px-2 py-1 text-xs hover:bg-slate-200"
											on:click={() => duplicateFont(font)}>Duplica</button
										>
										<button
											class="px-2 py-1 text-xs text-red-600 hover:bg-red-100"
											on:click={() => (deleteFontId = font.id)}>Elimina</button
										>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>

		<!-- Metadata Editor -->
		<div class="space-y-3">
			<div class="flex items-center gap-4">
				<p class="font-mono text-lg">Metadata font</p>
				<Button on:click={openMetadataEditor}>
					{editingMetadata ? 'Chiudi' : 'Modifica'}
				</Button>
			</div>

			{#if editingMetadata}
				<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaName">Nome</label>
						<input
							id="fontMetaName"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.name}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaFamily">Nome famiglia</label>
						<input
							id="fontMetaFamily"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.familyName}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaVersion">Versione</label>
						<input
							id="fontMetaVersion"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.version}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaCreated">Data di creazione</label>
						<input
							id="fontMetaCreated"
							type="date"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.createdDate}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaDesigner">Progettisti</label>
						<input
							id="fontMetaDesigner"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.designer}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaManufacturer">Produttore</label>
						<input
							id="fontMetaManufacturer"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.manufacturer}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaDesignerURL">URL progettista</label>
						<input
							id="fontMetaDesignerURL"
							type="url"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.designerURL}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaManufacturerURL">URL produttore</label>
						<input
							id="fontMetaManufacturerURL"
							type="url"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.manufacturerURL}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaLicense">Licenza</label>
						<textarea
							id="fontMetaLicense"
							class="min-h-20 w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={metadataForm.license}
						/>
					</div>
				</div>
				<Button on:click={saveMetadata}>Salva metadata</Button>
			{/if}
		</div>

		<!-- Export / Import -->
		<div class="space-y-3 border-t border-slate-200 pt-6">
			<p class="font-mono text-lg">Esporta / Importa</p>
			<div class="flex gap-3">
				<Button on:click={exportProjectArchive}>Salva Progetto (.chirone.tar.gz)</Button>
			</div>
			<div class="space-y-2">
				<p class="font-mono text-sm text-slate-600">
					Apri archivio .chirone.tar.gz o file GTL.json
				</p>
				<input
					type="file"
					accept=".chirone.tar.gz,.json"
					on:change={handleFileChange}
					class="font-mono text-sm"
				/>
				<div class="mt-1">
					<Upload on:upload={handleUpload}>Importa GTL.json</Upload>
				</div>
			</div>
		</div>
	</div>
</div>

<!-- Font Form Modal -->
<Modal
	outsideclose
	class="!rounded-none !font-mono"
	bind:open={fontFormOpen}
	title={editingFont ? 'Modifica font' : 'Nuovo font'}
>
	<div class="space-y-3 font-mono">
		<div class="space-y-1">
			<label class="text-sm" for="fontNameInput">Nome</label>
			<input
				id="fontNameInput"
				class="w-full border border-slate-400 px-3 py-2"
				placeholder="Regular"
				bind:value={fontName}
			/>
		</div>

		<div class="space-y-1">
			<label class="text-sm" for="fontSyntaxSelect">Syntax</label>
			<select
				id="fontSyntaxSelect"
				class="w-full border border-slate-400 bg-white px-3 py-2"
				bind:value={fontSyntaxId}
			>
				<option value="">— Seleziona syntax —</option>
				{#each $syntaxes as s (s.id)}
					<option value={s.id}>{s.name || s.id}</option>
				{/each}
			</select>
			{#if !fontSyntaxValid && fontSyntaxId}
				<p class="text-xs text-amber-600">Syntax non trovata</p>
			{/if}
		</div>

		<div class="space-y-1">
			<label class="text-sm" for="fontMetricsSelect">Metrics preset</label>
			<select
				id="fontMetricsSelect"
				class="w-full border border-slate-400 bg-white px-3 py-2"
				bind:value={fontMetricsId}
			>
				<option value="">— Seleziona metrics —</option>
				{#each $metricsPresets as m (m.id)}
					<option value={m.id}>{m.name} (UPM: {m.UPM}, h: {m.height})</option>
				{/each}
			</select>
			{#if !fontMetricsValid && fontMetricsId}
				<p class="text-xs text-amber-600">Metrics preset non trovato</p>
			{/if}
		</div>

		<div class="space-y-1">
			<label class="text-sm" for="fontMetaSelect">Metadata preset</label>
			<select
				id="fontMetaSelect"
				class="w-full border border-slate-400 bg-white px-3 py-2"
				bind:value={fontMetadataId}
			>
				<option value="">— Seleziona metadata —</option>
				{#each $metadataPresets as m (m.id)}
					<option value={m.id}>{m.name} ({m.familyName})</option>
				{/each}
			</select>
			{#if !fontMetadataValid && fontMetadataId}
				<p class="text-xs text-amber-600">Metadata preset non trovato</p>
			{/if}
		</div>

		<div class="space-y-1">
			<label class="text-sm" for="fontOutputName">Output filename</label>
			<input
				id="fontOutputName"
				class="w-full border border-slate-400 px-3 py-2"
				placeholder={deriveOutputName()}
				bind:value={fontOutputName}
			/>
		</div>

		<div class="flex items-center gap-2">
			<input type="checkbox" id="fontEnabled" bind:checked={fontEnabled} />
			<label class="text-sm" for="fontEnabled">Attivo</label>
		</div>

		<div class="flex gap-2 pt-2">
			<Button on:click={closeFontForm}>Annulla</Button>
			<Button disabled={!fontFormValid} on:click={saveFont}>
				{editingFont ? 'Aggiorna' : 'Crea'}
			</Button>
		</div>
	</div>
</Modal>

<!-- Delete Confirmation Modal -->
<Modal
	outsideclose
	class="!rounded-none !font-mono"
	bind:open={deleteModalOpen}
	title="Conferma eliminazione"
>
	<div class="space-y-3 font-mono">
		<p class="text-sm text-slate-800">
			Eliminare il font
			<span class="font-bold"
				>{$fontDefinitions.find((f) => f.id === deleteFontId)?.name ?? ''}</span
			>?
		</p>
		<div class="flex gap-2">
			<Button on:click={() => (deleteFontId = null)}>Annulla</Button>
			<button
				class="flex bg-red-700 p-3 font-mono text-white hover:bg-red-800"
				on:click={() => deleteFontId && confirmDeleteFont(deleteFontId)}
			>
				Elimina
			</button>
		</div>
	</div>
</Modal>

<!-- Import Summary Modal -->
<Modal
	outsideclose
	class="!rounded-none !font-mono"
	bind:open={importModalOpen}
	title="Riepilogo importazione"
>
	<div class="space-y-3 font-mono">
		<pre class="whitespace-pre-line text-sm text-slate-800">{importSummary}</pre>
		<p class="text-xs text-amber-700">
			Attenzione: l'importazione sostituirà il progetto corrente. Verrà scaricato automaticamente un
			backup.
		</p>
		<div class="flex gap-2">
			<Button on:click={() => (importModalOpen = false)}>Annulla</Button>
			<button
				class="flex bg-blue-700 p-3 font-mono text-white hover:bg-blue-800"
				on:click={confirmImport}
			>
				Importa (sostituisce corrente)
			</button>
		</div>
	</div>
</Modal>
