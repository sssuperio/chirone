<script lang="ts">
	import { nanoid } from 'nanoid';

	export let value: number;
	export let name = nanoid(5);
	export let step = 1;
</script>

<input type="number" bind:value class="w-20 bg-slate-200 p-2 font-mono" id={name} {name} {step} />
