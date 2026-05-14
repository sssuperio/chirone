<script lang="ts">
	import Button from './button.svelte';

	export let dataUrl: string;

	let loading = false;
	let files: FileList;

	$: {
		if (files) {
			loading = true;
			const file = files[0];
			const reader = new FileReader();
			reader.readAsDataURL(file);
			reader.onload = () => {
				const res = reader.result as string;
				dataUrl = res;
				loading = false;
			};
		}
	}
</script>

{#if !dataUrl}
	<input type="file" bind:files accept="image/svg+xml" />
{/if}
{#if loading}
	<p>Loading...</p>
{:else if dataUrl}
	<div class="flex w-fit flex-row items-center border-2 border-slate-300 bg-slate-300">
		<div class="bg-white">
			<img class="h-20 w-20 object-cover" src={dataUrl} alt="up" />
		</div>
		<Button
			on:click={() => {
				dataUrl = '';
			}}>x</Button
		>
	</div>
{/if}
