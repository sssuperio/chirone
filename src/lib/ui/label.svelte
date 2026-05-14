<script lang="ts">
	export let target: string;
</script>

<!--  -->

<label class="block font-mono text-sm text-slate-700" for={target}>
	<slot />
</label>
