<div
	id="sidebar-root"
	class="flex w-60 flex-shrink-0 flex-col flex-nowrap overflow-hidden bg-slate-100 p-6"
>
	<div id="sidebar-top" class="space-y-4 pb-4">
		<slot name="topArea" />

		{#if $$slots.topArea}
			<hr class="border-b-1 border-slate-300" />
		{/if}

		<p class="text-small mb-4 font-mono text-sm text-slate-900">
			<slot name="listTitle" />
		</p>
	</div>

	<div id="sidebar-list" class="h-0 grow space-y-1 overflow-y-auto">
		<slot name="items" />
	</div>
</div>
