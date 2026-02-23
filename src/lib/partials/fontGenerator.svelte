<script lang="ts">
	import { generateFont } from '$lib/GTL/createFont';
	import { metrics } from '$lib/stores';
	import type { GlyphInput, Syntax } from '$lib/types';

	export let syntax: Syntax;
	export let glyphs: Array<GlyphInput>;

	let fontPromise: Promise<opentype.Font>;
	$: fontPromise = generateFont(syntax, glyphs, $metrics);
</script>

{#await fontPromise}
	<p>loading</p>
{:then font}
	<slot {font} />
{:catch error}
	<p class="text-xs font-mono text-red-600">
		Errore generazione font: {error instanceof Error ? error.message : String(error)}
	</p>
{/await}
