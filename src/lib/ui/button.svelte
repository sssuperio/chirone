<script lang="ts">
	export let disabled = false;
</script>

<button
	class="flex bg-slate-300 p-3 font-mono text-slate-900 hover:bg-slate-400 disabled:opacity-40 disabled:hover:bg-slate-300"
	on:click
	{disabled}
>
	<slot />
</button>
