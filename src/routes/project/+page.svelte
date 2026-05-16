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
		saveGlyphsForFont,
		syncMetadataPreset
	} from '$lib/stores';
	import {
		createProjectArchive,
		parseProjectArchive,
		parseLegacyProjectFile,
		detectArchiveFormat
	} from '$lib/archive';
	import type { FontDefinition, GlyphInput } from '$lib/types';
	import {
		collabConfig,
		collabStatus,
		setCollabProject,
		setCollabServer,
		canOverrideCollabServer
	} from '$lib/collab/client';
	import Button from '$lib/ui/button.svelte';
	import Upload from '$lib/ui/upload.svelte';
	import { Modal } from 'flowbite-svelte';
	import { onMount } from 'svelte';
	import { base } from '$app/paths';
	import OnboardingWizard from '$lib/partials/onboardingWizard.svelte';

	let onboardingOpen = false;

	const collabServerPattern = /^(|\/|https?:\/\/.+)$/;
	let collabServerInput = $collabConfig.server;

	$: sanitizedCollabServerInput = collabServerInput.trim();
	$: isCollabServerValid = collabServerPattern.test(sanitizedCollabServerInput);
	$: canApplyCollabServer =
		isCollabServerValid && sanitizedCollabServerInput !== $collabConfig.server;

	function applyCollabServer() {
		if (!canApplyCollabServer) return;
		collabServerInput = setCollabServer(sanitizedCollabServerInput);
		loadServerProjects();
	}

	let serverProjects: Array<{
		project: string;
		version: number;
		hasPassword: boolean;
	}> = [];
	let projectsLoading = false;
	let projectsError = '';
	let showCreateProject = false;
	let newProjectName = '';
	let newProjectPassword = '';
	let adminPasswordInput = '';
	let creatingProject = false;

	async function loadServerProjects() {
		projectsLoading = true;
		projectsError = '';
		try {
			const base = $collabConfig.base;
			const res = await fetch(`${base}/api/projects`, { cache: 'no-store' });
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const data = await res.json();
			serverProjects = data.projects ?? [];
		} catch (e) {
			projectsError = e instanceof Error ? e.message : String(e);
		} finally {
			projectsLoading = false;
		}
	}

	async function switchToServerProject(id: string) {
		setCollabProject(id);
		projectInfo.set({
			...$projectInfo,
			name: id,
			updatedAt: new Date().toISOString()
		});
		// Bootstrap will apply the correct server state atomically via
		// applyRemoteSnapshot. Do not push defaults here — that would
		// overwrite server data before bootstrap loads it.
	}

	async function createServerProject() {
		if (!newProjectName.trim()) return;
		creatingProject = true;
		try {
			const base = $collabConfig.base;
			const res = await fetch(`${base}/api/project/create`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-Chirone-Admin-Password': adminPasswordInput
				},
				body: JSON.stringify({
					project: newProjectName.trim(),
					password: newProjectPassword || undefined
				})
			});
			if (!res.ok) {
				const text = await res.text();
				throw new Error(text || `HTTP ${res.status}`);
			}
			showCreateProject = false;
			const createdName = newProjectName.trim();
			newProjectName = '';
			newProjectPassword = '';
			adminPasswordInput = '';
			// Switch to the newly created project
			setCollabProject(createdName);
			projectInfo.set({
				...$projectInfo,
				name: createdName,
				updatedAt: new Date().toISOString()
			});
			loadServerProjects();
			onboardingOpen = true;
		} catch (e) {
			projectsError = e instanceof Error ? e.message : String(e);
		} finally {
			creatingProject = false;
		}
	}

	onMount(() => {
		if ($collabConfig.enabled) loadServerProjects();
		// Auto-open wizard for empty projects
		const projectKey = $collabConfig.enabled ? $collabStatus.project : $projectInfo.name;
		const flagKey = `onboarding-shown-${projectKey}`;
		const hasData = $fontDefinitions.length > 0 || $syntaxes.length > 0 || $glyphs.length > 0;
		if (!hasData && !localStorage.getItem(flagKey)) {
			onboardingOpen = true;
			localStorage.setItem(flagKey, '1');
		}
	});

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

	// Keep metadataPresets[0] in sync with fontMetadata so the preset
	// list always reflects the current metadata values.
	$: syncMetadataPreset($fontMetadata, $metadataPresets);
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

	async function exportProjectArchive() {
		try {
			const allFontGlyphs = new Map<string, GlyphInput[]>();
			allFontGlyphs.set($activeFontId || 'default', $glyphs);
			for (const font of $fontDefinitions) {
				if (font.id === $activeFontId) continue;
				const fontGlyphs = loadGlyphsForFont(font.id);
				if (fontGlyphs.length > 0) allFontGlyphs.set(font.id, fontGlyphs);
			}

			const blob = await createProjectArchive(
				allFontGlyphs,
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
				`Glifi: ${Array.from(parsed.perFontGlyphs.values()).reduce((sum, g) => sum + g.length, 0)}`,
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
				`Glifi: ${Array.from(parsed.perFontGlyphs.values()).reduce((sum, g) => sum + g.length, 0)}`,
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
		// Restore per-font glyphs
		const firstFontId = importData.fontDefinitions[0]?.id;
		if (firstFontId && importData.perFontGlyphs.has(firstFontId)) {
			glyphs.set(importData.perFontGlyphs.get(firstFontId)!);
		} else {
			glyphs.set([]);
		}
		for (const [fontId, fontGlyphs] of importData.perFontGlyphs) {
			if (fontId !== firstFontId) {
				saveGlyphsForFont(fontId, fontGlyphs);
			}
		}
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
			`Glifi: ${Array.from(parsed.perFontGlyphs.values()).reduce((sum, g) => sum + g.length, 0)}`,
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
		<!-- Server URL -->
		<div class="space-y-2 rounded border border-slate-200 p-4">
			<p class="font-mono text-sm font-semibold">Backend sync</p>
			<p class="font-mono text-xs text-slate-600">
				URL del server Chirone. Vuoto per disattivare, <code>/</code> per stesso binario.
			</p>
			<div class="flex items-center gap-2">
				<input
					class="w-full max-w-md border border-slate-400 px-3 py-2 font-mono text-sm"
					placeholder="http://localhost:8090"
					bind:value={collabServerInput}
				/>
				<Button disabled={!canApplyCollabServer} on:click={applyCollabServer}>Applica server</Button
				>
			</div>
			{#if !isCollabServerValid}
				<p class="font-mono text-xs text-rose-700">
					URL non valido. Usa http://..., https://..., / oppure vuoto.
				</p>
			{/if}
		</div>

		{#if $collabConfig.enabled}
			<div class="space-y-3 rounded border border-slate-200 p-4">
				<div class="flex items-center gap-3">
					<p class="font-mono text-sm font-semibold">Progetti sul server</p>
					<span class="rounded bg-slate-200 px-2 py-0.5 font-mono text-xs">
						{$collabStatus.project}
					</span>
					<Button on:click={loadServerProjects}>
						{projectsLoading ? '...' : 'Aggiorna'}
					</Button>
					<Button on:click={() => (showCreateProject = !showCreateProject)}>
						{showCreateProject ? 'Annulla' : '+ Nuovo'}
					</Button>
				</div>

				{#if showCreateProject}
					<div class="space-y-2 rounded bg-slate-50 p-3 font-mono">
						<p class="text-xs text-slate-600">Crea nuovo progetto (password admin richiesta)</p>
						<input
							class="w-full border border-slate-400 px-3 py-2 text-sm"
							placeholder="Nome progetto"
							bind:value={newProjectName}
						/>
						<input
							class="w-full border border-slate-400 px-3 py-2 text-sm"
							type="password"
							placeholder="Password admin"
							bind:value={adminPasswordInput}
						/>
						<input
							class="w-full border border-slate-400 px-3 py-2 text-sm"
							type="password"
							placeholder="Password progetto (opzionale)"
							bind:value={newProjectPassword}
						/>
						<Button
							disabled={!newProjectName.trim() || !adminPasswordInput || creatingProject}
							on:click={createServerProject}
						>
							{creatingProject ? 'Creazione...' : 'Crea progetto'}
						</Button>
					</div>
				{/if}

				{#if projectsError}
					<p class="font-mono text-xs text-red-600">{projectsError}</p>
				{/if}

				{#if serverProjects.length > 1}
					<div class="flex flex-wrap gap-1">
						{#each serverProjects as p (p.project)}
							<button
								class="rounded px-2 py-1 font-mono text-xs hover:bg-slate-200 {$collabStatus.project ===
								p.project
									? 'bg-slate-300 font-semibold'
									: 'bg-slate-100'}"
								on:click={() => switchToServerProject(p.project)}
							>
								{p.project}
								{#if p.hasPassword}
									🔒{/if}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		<!-- Project Info -->
		<div class="space-y-2">
			<p class="font-mono text-lg">Progetto</p>
			<div class="font-mono text-sm">
				<span class="text-slate-500">Nome: </span>
				<span class="font-semibold"
					>{$collabConfig.enabled ? $collabStatus.project : $projectInfo.name}</span
				>
			</div>
			<div class="flex gap-4 font-mono text-xs text-slate-400">
				<span>ID: {$projectInfo.id}</span>
			</div>
		</div>

		<!-- Font Definitions -->
		<div class="space-y-3">
			<div class="flex items-center gap-4">
				<p class="font-mono text-lg">Font</p>
				<Button on:click={() => (onboardingOpen = true)}>+ Nuovo font</Button>
			</div>

			{#if $fontDefinitions.length === 0}
				<p class="font-mono text-sm text-slate-500">
					Nessun font definito. Clicca &quot;+ Nuovo font&quot; per creare il primo font con il
					wizard guidato.
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
				<p class="font-mono text-lg">Metadata font</p>
				<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaName">Nome</label>
						<input
							id="fontMetaName"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.name}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaFamily">Nome famiglia</label>
						<input
							id="fontMetaFamily"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.familyName}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaVersion">Versione</label>
						<input
							id="fontMetaVersion"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.version}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaCreated">Data di creazione</label>
						<input
							id="fontMetaCreated"
							type="date"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.createdDate}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaDesigner">Progettisti</label>
						<input
							id="fontMetaDesigner"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.designer}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaManufacturer">Produttore</label>
						<input
							id="fontMetaManufacturer"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.manufacturer}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaDesignerURL">URL progettista</label>
						<input
							id="fontMetaDesignerURL"
							type="url"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.designerURL}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaManufacturerURL">URL produttore</label>
						<input
							id="fontMetaManufacturerURL"
							type="url"
							class="w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.manufacturerURL}
						/>
					</div>
					<div class="space-y-1">
						<label class="font-mono text-sm" for="fontMetaLicense">Licenza</label>
						<textarea
							id="fontMetaLicense"
							class="min-h-20 w-full border border-slate-400 px-3 py-2 font-mono"
							bind:value={$fontMetadata.license}
						/>
					</div>
				</div>
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

<OnboardingWizard bind:open={onboardingOpen} />

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
