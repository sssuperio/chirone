<script lang="ts">
	import { glyphs, syntaxes, metrics, fontMetadata } from '$lib/stores';
	import type { Syntax } from '$lib/types';

	import { generateFont } from '$lib/GTL/createFont';
	import { parsePreviewText } from '$lib/GTL/previewText';

	import FontTextPreview from '$lib/components/fontTextPreview.svelte';
	import FontGenerator from '$lib/partials/fontGenerator.svelte';

	import Label from '$lib/ui/label.svelte';
	import { previewText } from '$lib/stores';
	import Button from '$lib/ui/button.svelte';

	type PreviewPreset = {
		id: string;
		label: string;
		text: string;
		fontSize: number;
		fontFeatureSettings?: string;
	};

	const megaLoremIpsum = [
		'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed non risus. Suspendisse lectus tortor, dignissim sit amet, adipiscing nec, ultricies sed, dolor. Cras elementum ultrices diam. Maecenas ligula massa, varius a, semper congue, euismod non, mi.',
		'Proin porttitor, orci nec nonummy molestie, enim est eleifend mi, non fermentum diam nisl sit amet erat. Duis semper. Duis arcu massa, scelerisque vitae, consequat in, pretium a, enim. Pellentesque congue. Ut in risus volutpat libero pharetra tempor.',
		'Cras vestibulum bibendum augue. Praesent egestas leo in pede. Praesent blandit odio eu enim. Pellentesque sed dui ut augue blandit sodales. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Aliquam nibh.',
		'Mauris ac mauris sed pede pellentesque fermentum. Maecenas adipiscing ante non diam sodales hendrerit. Ut velit mauris, egestas sed, gravida nec, ornare ut, mi. Aenean ut orci vel massa suscipit pulvinar. Nulla sollicitudin.',
		'Fusce varius, ligula non tempus aliquam, nunc turpis ullamcorper nibh, in tempus sapien eros vitae ligula. Pellentesque rhoncus nunc et augue. Integer id felis. Curabitur aliquet pellentesque diam. Integer quis metus vitae elit lobortis egestas.',
		'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Morbi vel erat non mauris convallis vehicula. Nulla et sapien. Integer tortor tellus, aliquam faucibus, convallis id, congue eu, quam. Mauris ullamcorper felis vitae erat.',
		'Proin feugiat, augue non elementum posuere, metus purus iaculis lectus, et tristique ligula justo vitae magna. Aliquam convallis sollicitudin purus. Praesent aliquam, enim at fermentum mollis, ligula massa adipiscing nisl, ac euismod nibh nisl eu lectus.',
		'Nunc eu urna. Donec accumsan malesuada orci. Donec sit amet eros. Lorem ipsum dolor sit amet, consectetuer adipiscing elit. Mauris fermentum dictum magna. Sed laoreet aliquam leo. Ut tellus dolor, dapibus eget, elementum vel, cursus eleifend, elit.',
		'Aenean auctor wisi et urna. Aliquam erat volutpat. Duis ac turpis. Integer rutrum ante eu lacus. Vestibulum libero nisl, porta vel, scelerisque eget, malesuada at, neque. Vivamus eget nibh. Etiam cursus leo vel metus.',
		'Nulla facilisi. Aenean nec eros. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Suspendisse sollicitudin velit sed leo. Ut pharetra augue nec augue. Nam elit magna, hendrerit sit amet, tincidunt ac, viverra sed, nulla.',
		'Donec porta diam eu massa. Quisque diam lorem, interdum vitae, dapibus ac, scelerisque vitae, pede. Donec eget tellus non erat lacinia fermentum. Donec in velit vel ipsum auctor pulvinar. Vestibulum iaculis lacinia est. Proin dictum elementum velit.',
		'Fusce euismod consequat ante. Lorem ipsum dolor sit amet, consectetuer adipiscing elit. Pellentesque sed dolor. Aliquam congue fermentum nisl. Mauris accumsan nulla vel diam. Sed in lacus ut enim adipiscing aliquet.',
		'Nulla venenatis. In pede mi, aliquet sit amet, euismod in, auctor ut, ligula. Aliquam dapibus tincidunt metus. Praesent justo dolor, lobortis quis, lobortis dignissim, pulvinar ac, lorem. Vestibulum sed ante.',
		'Donec sagittis euismod purus. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo.',
		'Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt. Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit.',
		'Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam, nisi ut aliquid ex ea commodi consequatur. Quis autem vel eum iure reprehenderit qui in ea voluptate velit esse quam nihil molestiae consequatur.',
		'Vel illum qui dolorem eum fugiat quo voluptas nulla pariatur? At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium voluptatum deleniti atque corrupti quos dolores.',
		'Et quas molestias excepturi sint occaecati cupiditate non provident, similique sunt in culpa qui officia deserunt mollitia animi, id est laborum et dolorum fuga. 0123456789, !?.,;: - [] {} () / \\ @ # & * + = _',
		'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed non risus. Suspendisse lectus tortor, dignissim sit amet, adipiscing nec, ultricies sed, dolor. Cras elementum ultrices diam. Maecenas ligula massa, varius a, semper congue, euismod non, mi.',
		'Proin porttitor, orci nec nonummy molestie, enim est eleifend mi, non fermentum diam nisl sit amet erat. Duis semper. Duis arcu massa, scelerisque vitae, consequat in, pretium a, enim. Pellentesque congue. Ut in risus volutpat libero pharetra tempor.',
		'Cras vestibulum bibendum augue. Praesent egestas leo in pede. Praesent blandit odio eu enim. Pellentesque sed dui ut augue blandit sodales. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Aliquam nibh.',
		'Mauris ac mauris sed pede pellentesque fermentum. Maecenas adipiscing ante non diam sodales hendrerit. Ut velit mauris, egestas sed, gravida nec, ornare ut, mi. Aenean ut orci vel massa suscipit pulvinar. Nulla sollicitudin.',
		'Fusce varius, ligula non tempus aliquam, nunc turpis ullamcorper nibh, in tempus sapien eros vitae ligula. Pellentesque rhoncus nunc et augue. Integer id felis. Curabitur aliquet pellentesque diam. Integer quis metus vitae elit lobortis egestas.'
	].join('\n\n');

	const latinBasePreview = [
		'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
		'abcdefghijklmnopqrstuvwxyz',
		'0123456789',
		'Aa Bb Cc Dd Ee Ff Gg Hh Ii Jj Kk Ll Mm',
		'Nn Oo Pp Qq Rr Ss Tt Uu Vv Ww Xx Yy Zz'
	].join('\n');

	const previewPresets: Array<PreviewPreset> = [
		{
			id: 'latin-base',
			label: 'Latin base',
			text: latinBasePreview,
			fontSize: 96
		},
		{
			id: 'title',
			label: 'Title',
			text: 'The Quick Brown Fox',
			fontSize: 140
		},
		{
			id: 'paragraph',
			label: 'Paragraph',
			text: megaLoremIpsum,
			fontSize: 68
		},
		{
			id: 'symbols',
			label: 'Symbols + punctuation',
			text: '.,;:!? () [] {} / \\\\ - _ + = * @ # & % 0123456789',
			fontSize: 86
		},
		{
			id: 'ss01',
			label: 'Stylistic set ss01',
			text: 'abcdefghijklmnopqrstuvwxyz\nABCDEFGHIJKLMNOPQRSTUVWXYZ',
			fontSize: 120,
			fontFeatureSettings: '"ss01" 1'
		},
		{
			id: 'ss02',
			label: 'Stylistic set ss02',
			text: 'abcdefghijklmnopqrstuvwxyz\nABCDEFGHIJKLMNOPQRSTUVWXYZ',
			fontSize: 120,
			fontFeatureSettings: '"ss02" 1'
		},
		{
			id: 'stylistic-mix',
			label: 'Stylistic mix',
			text: 'Sphinx of black quartz, judge my vow.\nWaltz, bad nymph, for quick jigs vex.\n0123456789',
			fontSize: 108,
			fontFeatureSettings: '"ss01" 1, "ss02" 1, "ss03" 1'
		}
	];

	let previewHasNamedTokens = false;
	let validText = '';
	let downloadError = '';
	let previewPresetID = 'custom';
	let previewFontSize = 120;
	let previewFeatureSettings = '"liga" 1, "rlig" 1';

	$: {
		const parsed = parsePreviewText($previewText, $glyphs);
		previewHasNamedTokens = parsed.hasNamedTokens;
		validText = parsed.validText;
	}

	function applyPresetByID(presetID: string) {
		const preset = previewPresets.find((candidate) => candidate.id === presetID);
		if (!preset) return;

		$previewText = preset.text;
		previewFontSize = preset.fontSize;
		previewFeatureSettings = preset.fontFeatureSettings ?? '"liga" 1, "rlig" 1';
	}

	function handlePresetChange(event: Event) {
		const value = (event.target as HTMLSelectElement).value;
		previewPresetID = value;
		if (value === 'custom') return;
		applyPresetByID(value);
	}

	function toSafeFileSegment(input: string): string {
		const cleaned = input.trim().replace(/\s+/g, '-');
		return cleaned.replace(/[^a-zA-Z0-9._-]/g, '');
	}

	async function downloadFont(s: Syntax) {
		downloadError = '';

		try {
			const font = await generateFont(s, $glyphs, $metrics, $fontMetadata);
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

			window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
		} catch (error) {
			downloadError = error instanceof Error ? error.message : String(error);
			console.error('downloadFont failed', error);
		}
	}
</script>

<div class="grow flex flex-col items-stretch overflow-x-hidden overflow-y-auto">
	<div
		class="p-8 space-y-2 overflow-x-hidden shrink-0 sticky top-0 border-b border-b-gray-200 bg-white"
	>
		<div class="flex flex-wrap items-end gap-4">
			<div class="space-y-1">
				<Label target="previewPreset">Preset</Label>
				<select
					id="previewPreset"
					class="h-10 min-w-52 bg-slate-200 px-2 font-mono text-sm"
					value={previewPresetID}
					on:change={handlePresetChange}
				>
					<option value="custom">Custom</option>
					{#each previewPresets as preset (preset.id)}
						<option value={preset.id}>{preset.label}</option>
					{/each}
				</select>
			</div>

			<div class="space-y-1">
				<Label target="previewFontSize">Font size: {previewFontSize}</Label>
				<input
					id="previewFontSize"
					name="previewFontSize"
					type="range"
					min="0"
					max="800"
					step="1"
					bind:value={previewFontSize}
					class="w-72 accent-slate-900"
				/>
			</div>
		</div>

		<Label target="previewText">Preview text</Label>
		<textarea
			id="previewText"
			name="previewText"
			class="w-full min-h-32 resize-y border border-slate-300 bg-white p-3 font-mono text-sm"
			bind:value={$previewText}
		/>
		<p class="text-xs font-mono text-slate-500">
			Usa `/nomeGlifo` per forzare un glifo (es: `Cia/a.ss01o`, `/etom.component/a`).
		</p>
		{#if previewHasNamedTokens}
			<p class="text-xs font-mono text-amber-700">
				In modalita CSS i token `/nomeGlifo` sono mostrati come fallback testuale.
			</p>
		{/if}
	</div>

	<div class="p-8 space-y-8">
		{#if downloadError}
			<p class="text-sm font-mono text-red-600">Errore download: {downloadError}</p>
		{/if}

		{#each $syntaxes as syntax (syntax.name)}
			<FontGenerator glyphs={$glyphs} {syntax} metadata={$fontMetadata} let:font>
				{#if font}
					<div class="space-y-4 min-h-[72vh] flex flex-col">
						<div class="flex flex-row flex-nowrap items-center space-x-8">
							<h2 class="text-lg font-mono">{font.names.fullName.en}</h2>
							<Button
								on:click={() => {
									downloadFont(syntax);
								}}>↓ Download</Button
							>
						</div>
						<div class="grow min-h-0">
							<FontTextPreview
								{font}
								text={previewHasNamedTokens ? validText : $previewText}
								fontSize={previewFontSize}
								fontFeatureSettings={previewFeatureSettings}
								className="h-full"
							/>
						</div>
					</div>
				{/if}
			</FontGenerator>
		{/each}
	</div>
</div>
