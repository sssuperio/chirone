<script lang="ts">
	import { glyphs as glyphsStore } from '$lib/stores';
	import type { GlyphInput } from '$lib/types';
	import { parsePreviewText } from '$lib/GTL/previewText';

	import InputText from '$lib/ui/inputText.svelte';

	/**
	 * Generating font previews
	 */

	export let text = '';
	export let glyphs: Array<GlyphInput> = [];
	export let validText = '';
	export let glyphSequence: Array<string> = [];
	export let hasNamedTokens = false;
	export let name = 'previewText';

	$: {
		const parsed = parsePreviewText(text, $glyphsStore);
		glyphs = parsed.foundGlyphs;
		validText = parsed.validText;
		glyphSequence = parsed.glyphSequence;
		hasNamedTokens = parsed.hasNamedTokens;
	}
</script>

<!--  -->

<InputText {name} bind:value={text} grow />
