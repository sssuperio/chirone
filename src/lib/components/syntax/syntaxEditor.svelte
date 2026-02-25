<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import {
		parseGlyphStructure,
		replaceGlyphStructureBody,
		replaceGlyphStructureComponents
	} from '$lib/GTL/structure';
	import { ShapeKind, type GlyphInput, type Rule, type Syntax } from '$lib/types';
	import Button from '$lib/ui/button.svelte';
	import SyntaxRule from './syntaxRule.svelte';

	export let syntax: Syntax;
	export let glyphs: Array<GlyphInput> = [];

	const dispatch = createEventDispatcher<{
		changed: { glyphsChanged: boolean };
		voidLimit: { symbol: string };
	}>();

	let pendingDeleteSymbol: string | undefined;
	let replacementSymbol = '';
	let voidLimitMessage = '';

	function rowHasSymbol(row: string, symbol: string): boolean {
		for (const char of Array.from(row)) {
			if (char === symbol) return true;
		}
		return false;
	}

	function getAffectedGlyphNames(symbol: string): Array<string> {
		if (!symbol) return [];
		const affected: Array<string> = [];
		for (const glyph of glyphs) {
			const parsed = parseGlyphStructure(glyph.structure);
			const rows = parsed.body.split(/\r?\n/);
			const usedInBody = rows.some((row) => rowHasSymbol(row, symbol));
			const usedInComponents = parsed.components.some((component) => component.symbol === symbol);
			if (usedInBody || usedInComponents) {
				affected.push(glyph.name);
			}
		}
		return affected;
	}

	function replaceSymbolInBody(body: string, currentSymbol: string, nextSymbol: string): string {
		const rows = body.split(/\r?\n/);
		const replacedRows = rows.map((row) =>
			Array.from(row)
				.map((char) => (char === currentSymbol ? nextSymbol : char))
				.join('')
		);
		return replacedRows.join('\n');
	}

	function canRuleUseVoidShape(rule: Rule): boolean {
		if (rule.shape.kind === ShapeKind.Void) return true;
		return !syntax.rules.some((candidate) => candidate !== rule && candidate.shape.kind === ShapeKind.Void);
	}

	$: pendingDeleteRule = pendingDeleteSymbol
		? syntax.rules.find((rule) => rule.symbol === pendingDeleteSymbol)
		: undefined;
	$: affectedGlyphNames =
		pendingDeleteRule && pendingDeleteSymbol ? getAffectedGlyphNames(pendingDeleteSymbol) : [];
	$: replacementOptions = pendingDeleteRule
		? syntax.rules
				.filter((rule) => rule.symbol !== pendingDeleteRule.symbol)
				.map((rule) => ({
					label: rule.symbol,
					value: rule.symbol
				}))
		: [];
	$: if (
		pendingDeleteRule &&
		!replacementOptions.some((option) => option.value === replacementSymbol)
	) {
		replacementSymbol = replacementOptions[0]?.value ?? '';
	}

	function requestRuleDelete(symbol: string) {
		voidLimitMessage = '';
		pendingDeleteSymbol = symbol;
	}

	function cancelRuleDelete() {
		pendingDeleteSymbol = undefined;
		replacementSymbol = '';
	}

	function confirmRuleDelete() {
		if (!pendingDeleteRule || !pendingDeleteSymbol) return;

		const targetSymbol = pendingDeleteSymbol;
		const usedByGlyphs = getAffectedGlyphNames(targetSymbol);
		const mustReplace = usedByGlyphs.length > 0;
		if (mustReplace && !replacementSymbol) return;

		let glyphsChanged = false;
		if (mustReplace && replacementSymbol) {
			for (const glyph of glyphs) {
				const parsed = parseGlyphStructure(glyph.structure);
				const replacedBody = replaceSymbolInBody(parsed.body, targetSymbol, replacementSymbol);
				const replacedComponents = parsed.components.map((component) =>
					component.symbol === targetSymbol
						? {
								...component,
								symbol: replacementSymbol
							}
						: component
				);
				const bodyChanged = replacedBody !== parsed.body;
				const componentChanged = replacedComponents.some(
					(component, index) => component.symbol !== parsed.components[index].symbol
				);

				if (bodyChanged || componentChanged) {
					let nextStructure = glyph.structure;
					if (bodyChanged) {
						nextStructure = replaceGlyphStructureBody(nextStructure, replacedBody);
					}
					if (componentChanged) {
						nextStructure = replaceGlyphStructureComponents(nextStructure, replacedComponents);
					}
					glyph.structure = nextStructure;
					glyphsChanged = true;
				}
			}
		}

		const nextRules = syntax.rules.filter((rule) => rule.symbol !== targetSymbol);
		if (nextRules.length === syntax.rules.length) {
			cancelRuleDelete();
			return;
		}

		syntax.rules = nextRules;
		dispatch('changed', { glyphsChanged });
		cancelRuleDelete();
	}

	function handleVoidLimit(event: CustomEvent<{ symbol: string }>) {
		voidLimitMessage = `Solo una regola "void" è consentita per sintassi. Simbolo: ${event.detail.symbol}`;
		dispatch('voidLimit', event.detail);
	}
</script>

<p class="text-small mb-2 font-mono text-sm text-slate-900">Regole sintassi</p>
{#if voidLimitMessage}
	<p class="mb-4 border border-amber-300 bg-amber-100 px-3 py-2 font-mono text-xs text-slate-900">
		{voidLimitMessage}
	</p>
{/if}
<div class="flex flex-wrap items-start gap-6">
	{#if syntax.rules.length}
		{#each syntax.rules as rule (rule.symbol)}
			<div class="w-full min-w-[20rem] basis-[24rem] flex-1 space-y-3 border border-slate-200 bg-white p-4">
				<SyntaxRule
					bind:rule
					canUseVoidShape={canRuleUseVoidShape(rule)}
					on:voidLimit={handleVoidLimit}
				/>

				{#if pendingDeleteSymbol === rule.symbol}
					<div class="space-y-3 border border-slate-300 bg-slate-50 p-3 font-mono text-xs">
						<p class="text-slate-900">
							Eliminare regola `{rule.symbol}`?
						</p>
						{#if affectedGlyphNames.length}
							<p class="text-slate-700">
								Glifi coinvolti ({affectedGlyphNames.length}): {affectedGlyphNames.join(', ')}
							</p>
						{:else}
							<p class="text-slate-500">Nessun glifo contiene questo simbolo.</p>
						{/if}

						<div class="space-y-1">
							<label class="text-slate-700" for={`replace-rule-${rule.symbol}`}>
								Sostituisci con simbolo
							</label>
							<select
								id={`replace-rule-${rule.symbol}`}
								class="h-10 w-full bg-slate-200 px-2"
								bind:value={replacementSymbol}
								disabled={!replacementOptions.length}
							>
								{#if !replacementOptions.length}
									<option value="">Nessuna regola disponibile</option>
								{/if}
								{#each replacementOptions as option (option.value)}
									<option value={option.value}>{option.label}</option>
								{/each}
							</select>
						</div>

						{#if affectedGlyphNames.length && !replacementOptions.length}
							<p class="text-rose-700">
								Impossibile eliminare: i glifi usano questo simbolo e non c'è una regola alternativa.
							</p>
						{/if}

						<div class="flex items-center gap-2">
							<Button
								disabled={affectedGlyphNames.length > 0 && !replacementOptions.length}
								on:click={confirmRuleDelete}
							>
								Conferma elimina
							</Button>
							<Button on:click={cancelRuleDelete}>Annulla</Button>
						</div>
					</div>
				{:else}
					<Button on:click={() => requestRuleDelete(rule.symbol)}>
						[x] Elimina regola `{rule.symbol}`
					</Button>
				{/if}
			</div>
		{/each}
	{:else}
		<p class="w-full border-2 border-slate-200 p-12 font-mono text-slate-300">
			Inizia a disegnare un glifo per modificare la sintassi!
		</p>
	{/if}
</div>
