<script lang="ts">
	import { glyphs, syntaxes, metrics } from '$lib/stores';
	import type { GlyphInput, Syntax } from '$lib/types';

	import { generateFont } from '$lib/GTL/createFont';

	import FontDisplay from '$lib/components/fontDisplay.svelte';
	import FontGenerator from '$lib/partials/fontGenerator.svelte';

	import Label from '$lib/ui/label.svelte';
	import { previewText } from '$lib/stores';
	import Button from '$lib/ui/button.svelte';
	import GlyphsField from '$lib/partials/glyphsField.svelte';

	//

	let previewGlyphs: Array<GlyphInput> = [];
	let validText = '';
	let downloadError = '';

	function toSafeFileSegment(input: string): string {
		const cleaned = input.trim().replace(/\s+/g, '-');
		return cleaned.replace(/[^a-zA-Z0-9._-]/g, '');
	}

	async function downloadFont(s: Syntax) {
		downloadError = '';

		try {
			const font = await generateFont(s, $glyphs, $metrics);
			const fileName = `GTL-${toSafeFileSegment(s.name) || 'style'}.otf`;
			const buffer = font.toArrayBuffer();
			const blob = new Blob([buffer], { type: 'font/otf' });
			const objectUrl = URL.createObjectURL(blob);

			const link = document.createElement('a');
			link.href = objectUrl;
			link.download = fileName;
			document.body.appendChild(link);
			link.click();
			link.remove();

			// Revoke asynchronously to avoid cancelling the download in some browsers.
			window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
		} catch (error) {
			downloadError = error instanceof Error ? error.message : String(error);
			console.error('downloadFont failed', error);
		}
	}
</script>

<!--  -->

<div class="grow flex flex-col items-stretch overflow-x-hidden overflow-y-auto">
	<div
		class="p-8 space-y-2 overflow-x-hidden shrink-0 sticky top-0 border-b border-b-gray-200 bg-white"
	>
		<Label target="previewText">Preview text</Label>
		<GlyphsField
			name="previewText"
			bind:text={$previewText}
			bind:glyphs={previewGlyphs}
			bind:validText
		/>
	</div>

	<div class="p-8 space-y-8">
		{#if downloadError}
			<p class="text-sm font-mono text-red-600">Errore download: {downloadError}</p>
		{/if}

		{#each $syntaxes as syntax (syntax.name)}
			<FontGenerator glyphs={previewGlyphs} {syntax} let:font>
				{#if font}
					<div class="space-y-4">
						<div class="flex flex-row flex-nowrap items-center space-x-8">
							<h2 class="text-lg font-mono">{font.names.fullName.en}</h2>
							<Button
								on:click={() => {
									downloadFont(syntax);
								}}>↓ Download</Button
							>
						</div>
						<FontDisplay {font} bind:text={validText} />
					</div>
				{/if}
			</FontGenerator>
		{/each}
	</div>
</div>
