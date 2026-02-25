<script lang="ts">
	import { onDestroy } from 'svelte';
	import type opentype from 'opentype.js';

	export let font: opentype.Font;
	export let text = '';
	export let fontSize = 72;
	export let lineHeight = 1.35;
	export let letterSpacing = 0;
	export let fontFeatureSettings = '"liga" 1, "rlig" 1';
	export let className = '';

	let fontFamily = '';
	let fontURL = '';
	let fontError = '';
	let activeFace: FontFace | undefined;
	let loadCounter = 0;
	let effectiveFontSize = 72;

	$: effectiveFontSize = fontSize <= 0 ? 96 : Math.max(1, fontSize);

	function clearLoadedFace() {
		if (typeof document !== 'undefined' && activeFace) {
			document.fonts.delete(activeFace);
		}
		activeFace = undefined;
		fontFamily = '';
		if (fontURL) {
			URL.revokeObjectURL(fontURL);
			fontURL = '';
		}
	}

	async function loadFace(sourceFont: opentype.Font) {
		if (typeof window === 'undefined' || typeof document === 'undefined') return;
		const token = ++loadCounter;
		fontError = '';
		clearLoadedFace();

		try {
			const blob = new Blob([sourceFont.toArrayBuffer()], { type: 'font/otf' });
			const url = URL.createObjectURL(blob);
			const family = `GTLPreview_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
			const face = new FontFace(family, `url(${url})`);
			await face.load();

			if (token !== loadCounter) {
				URL.revokeObjectURL(url);
				return;
			}

			document.fonts.add(face);
			activeFace = face;
			fontFamily = family;
			fontURL = url;
		} catch (error) {
			fontError = error instanceof Error ? error.message : String(error);
		}
	}

	$: if (font) {
		void loadFace(font);
	}

	onDestroy(() => {
		clearLoadedFace();
	});
</script>

<div class={`h-full w-full rounded border border-slate-200 bg-white ${className}`}>
	{#if fontError}
		<p class="p-4 text-xs font-mono text-red-600">Errore preview font: {fontError}</p>
	{:else}
		<div
			class="h-full w-full overflow-auto p-8"
			style={`font-family: ${fontFamily ? `'${fontFamily}', ` : ''}monospace; font-size: ${effectiveFontSize}px; line-height: ${Math.max(0.8, lineHeight)}; letter-spacing: ${letterSpacing}px; font-feature-settings: ${fontFeatureSettings}; white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-word;`}
		>
			{text}
		</div>
	{/if}
</div>
