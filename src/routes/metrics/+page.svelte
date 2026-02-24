<script lang="ts">
	import { metrics } from '$lib/stores';
	import { areMetricsEqual, estimateVerticalMetrics, normalizeFontMetrics } from '$lib/GTL/metrics';

	import InputNumber from '$lib/ui/inputNumber.svelte';
	import Label from '$lib/ui/label.svelte';
	import Button from '$lib/ui/button.svelte';

	let lastHeight = 0;
	let lastDescender = 0;

	function applyEstimatedVerticals() {
		const estimated = estimateVerticalMetrics($metrics.height, $metrics.descender);
		const next = normalizeFontMetrics({
			...$metrics,
			height: estimated.height,
			descender: estimated.descender,
			baseline: estimated.descender,
			ascender: estimated.ascender,
			capHeight: estimated.capHeight,
			xHeight: estimated.xHeight
		});
		if (!areMetricsEqual($metrics, next)) {
			$metrics = next;
		}
		lastHeight = next.height;
		lastDescender = next.descender;
	}

	$: {
		const normalized = normalizeFontMetrics($metrics);
		if (!areMetricsEqual($metrics, normalized)) {
			$metrics = normalized;
		}
	}

	$: if ($metrics.height !== lastHeight || $metrics.descender !== lastDescender) {
		applyEstimatedVerticals();
	}
</script>

<div class="p-8 space-y-8">
	<div class="space-y-2">
		<p class="font-mono text-sm text-slate-700">
			Metriche discrete su celle. `height` e `descender` aggiornano automaticamente ascender/cap/x-height.
		</p>
		<Button on:click={applyEstimatedVerticals}>Ricalcola stime default</Button>
	</div>

	<div class="grid grid-cols-2 md:grid-cols-3 gap-4">
		<div class="flex flex-col">
			<Label target="UPM">UPM</Label>
			<InputNumber name="UPM" bind:value={$metrics.UPM} />
		</div>

		<div class="flex flex-col">
			<Label target="height">height (celle)</Label>
			<InputNumber name="height" bind:value={$metrics.height} />
		</div>

		<div class="flex flex-col">
			<Label target="descender">descender (celle)</Label>
			<InputNumber name="descender" bind:value={$metrics.descender} />
		</div>

		<div class="flex flex-col">
			<Label target="ascender">ascender (celle)</Label>
			<input
				id="ascender"
				name="ascender"
				class="w-20 p-2 bg-slate-100 font-mono text-slate-700"
				type="number"
				value={$metrics.ascender}
				readonly
			/>
		</div>

		<div class="flex flex-col">
			<Label target="capHeight">capHeight (celle)</Label>
			<InputNumber name="capHeight" bind:value={$metrics.capHeight} />
		</div>

		<div class="flex flex-col">
			<Label target="xHeight">xHeight (celle)</Label>
			<InputNumber name="xHeight" bind:value={$metrics.xHeight} />
		</div>
	</div>
</div>
