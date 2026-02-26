<script lang="ts">
	import { onMount } from 'svelte';
	import type { GlyphInput, Rule, Syntax } from '$lib/types';
	import { glyphs, metrics, selectedGlyph, syntaxes } from '$lib/stores';

	import Sidebar from '$lib/ui/sidebar.svelte';
	import SidebarTile from '$lib/ui/sidebarTile.svelte';
	import Button from '$lib/ui/button.svelte';
	import GlyphPainter from '$lib/components/glyph/glyphPainter.svelte';
	import RuleShapePreview from '$lib/components/glyph/ruleShapePreview.svelte';
	import GlyphPreview from '$lib/partials/glyphPreview.svelte';

	import { ShapeKind, createEmptyRule } from '$lib/types';
	import DeleteButton from '$lib/ui/deleteButton.svelte';
	import AddGlyphModal from './AddGlyphModal.svelte';
	import AddGlyphSetModal from './AddGlyphSetModal.svelte';
	import { resolveUnicodeNumber } from '$lib/GTL/glyphName';
	import {
		getGlyphSetByID,
		getGlyphSetDefinitions,
		inferGlyphSetID
	} from '$lib/GTL/glyphSets';
	import {
		isComponentGlyphName,
		parseGlyphStructure,
		replaceGlyphStructureBody,
		replaceGlyphStructureComponents,
		resolveGlyphStructures,
		resolveGlyphStructuresWithComponentMask,
		serializeGlyphStructure,
		type GlyphComponentRef
	} from '$lib/GTL/structure';
	import {
		getCrossedQuarterTurns,
		mapDirectionalSymbolsForRotation,
		reflectGlyphStructureBody,
		replaceMappedSymbols,
		rotateGlyphStructureBody
	} from '$lib/GTL/structureTransforms';

	//

	function getUniqueSymbols(
		glyphs: Array<GlyphInput>,
		resolvedTextByName: Map<string, string>,
		resolvedVisualByName: Map<string, string>
	): Array<string> {
		const symbols = new Set<string>();
		for (const glyph of glyphs) {
			const parsed = parseGlyphStructure(glyph.structure);
			const textStructure = resolvedTextByName.get(glyph.name) ?? parsed.body;
			const visualStructure = resolvedVisualByName.get(glyph.name) ?? parsed.body;
			for (const char of Array.from(textStructure.replace(/\n/g, ''))) {
				symbols.add(char);
			}
			for (const char of Array.from(visualStructure.replace(/\n/g, ''))) {
				symbols.add(char);
			}
		}
		return Array.from(symbols);
	}

	function updateSyntaxSymbols(syntax: Syntax, symbols: Array<string>): boolean {
		let changed = false;
		const usedSymbols = new Set(symbols);

		// Getting all symbols in syntax
		const syntaxSymbols = syntax.rules.map((r) => r.symbol);
		// Checking for additions
		for (let symbol of symbols) {
			if (!syntaxSymbols.includes(symbol)) {
				const newRule = createEmptyRule(symbol);
				newRule.unused = false;
				syntax.rules.push(newRule);
				changed = true;
			}
		}

		// Keep rules, but mark whether they are currently unused in glyphs.
		for (const rule of syntax.rules) {
			const nextUnused = !usedSymbols.has(rule.symbol);
			if ((rule.unused ?? false) !== nextUnused) {
				rule.unused = nextUnused;
				changed = true;
			}
		}

		return changed;
	}

	$: resolvedGlyphStructuresText = resolveGlyphStructures($glyphs, {
		transparentSymbols: [' ', '.']
	});
	$: resolvedGlyphStructuresVisualData = resolveGlyphStructuresWithComponentMask($glyphs, {
		transparentSymbols: [' ', '.'],
		applySymbolOverride: false
	});
	$: resolvedGlyphStructuresTextForDisplay = resolveGlyphStructures($glyphs, {
		transparentSymbols: [' ', '.'],
		rulesBySymbol: getRulesBySymbol($syntaxes)
	});
	$: resolvedGlyphStructuresComponentSymbolTextForDisplay = resolveGlyphStructures($glyphs, {
		transparentSymbols: [' '],
		rulesBySymbol: getRulesBySymbol($syntaxes)
	});
	$: resolvedGlyphStructuresVisualDataForDisplay = resolveGlyphStructuresWithComponentMask($glyphs, {
		transparentSymbols: [' ', '.'],
		applySymbolOverride: false,
		rulesBySymbol: getRulesBySymbol($syntaxes)
	});
	$: resolvedGlyphStructuresVisual = new Map(
		Array.from(resolvedGlyphStructuresVisualData.entries()).map(([name, resolved]) => [
			name,
			resolved.body
		])
	);
	$: {
		const uniqueSymbols = getUniqueSymbols(
			$glyphs,
			resolvedGlyphStructuresText,
			resolvedGlyphStructuresVisual
		);
		let changed = false;
		for (let syntax of $syntaxes) {
			if (updateSyntaxSymbols(syntax, uniqueSymbols)) {
				changed = true;
			}
		}
		if (changed) {
			$syntaxes = [...$syntaxes];
		}
	}

	function handleDelete() {
		$glyphs = $glyphs.filter((g) => g.id != $selectedGlyph);
		$selectedGlyph = $glyphs[0].id;
	}

	function sortGlyphs(glyphs: GlyphInput[]): GlyphInput[] {
		return [...glyphs].sort((a, b) => {
			const aUnicode = resolveUnicodeNumber(a.name);
			const bUnicode = resolveUnicodeNumber(b.name);

			if (aUnicode !== undefined && bUnicode !== undefined) {
				return aUnicode - bUnicode;
			}
			if (aUnicode !== undefined) return -1;
			if (bUnicode !== undefined) return 1;

			return a.name.localeCompare(b.name);
		});
	}

	function getGlyphString(glyphName: string): string | undefined {
		const unicode = resolveUnicodeNumber(glyphName);
		if (unicode === undefined) return undefined;
		try {
			return String.fromCodePoint(unicode);
		} catch {
			return undefined;
		}
	}

	function getGlyphUnicodeHex(glyphName: string): string | undefined {
		const unicode = resolveUnicodeNumber(glyphName);
		if (unicode === undefined) return undefined;
		return unicode.toString(16).toUpperCase().padStart(4, '0');
	}

	function isPreviewFilledCell(char: string): boolean {
		return char !== ' ' && char !== '.';
	}

	function getMiniGlyphPreviewSymbols(structure: string, maxRows = 8, maxCols = 8): Array<Array<string>> {
		const rows = (structure || '').split(/\r?\n/);
		const height = rows.length;
		const width = Math.max(0, ...rows.map((row) => row.length));

		if (!height || !width) return [[' ']];

		let minRow = height;
		let maxRow = -1;
		let minCol = width;
		let maxCol = -1;

		for (let row = 0; row < height; row++) {
			const line = rows[row] ?? '';
			for (let col = 0; col < width; col++) {
				const char = line[col] ?? ' ';
				if (!isPreviewFilledCell(char)) continue;
				if (row < minRow) minRow = row;
				if (row > maxRow) maxRow = row;
				if (col < minCol) minCol = col;
				if (col > maxCol) maxCol = col;
			}
		}

		if (maxRow < minRow || maxCol < minCol) return [[' ']];

		const cropHeight = maxRow - minRow + 1;
		const cropWidth = maxCol - minCol + 1;
		const targetRows = Math.max(1, Math.min(maxRows, cropHeight));
		const targetCols = Math.max(1, Math.min(maxCols, cropWidth));

		const preview: Array<Array<string>> = Array.from({ length: targetRows }, () =>
			Array.from({ length: targetCols }, () => ' ')
		);

		for (let tr = 0; tr < targetRows; tr++) {
			const sourceRowStart = Math.floor((tr * cropHeight) / targetRows);
			const sourceRowEndExclusive = Math.floor(((tr + 1) * cropHeight) / targetRows);

			for (let tc = 0; tc < targetCols; tc++) {
				const sourceColStart = Math.floor((tc * cropWidth) / targetCols);
				const sourceColEndExclusive = Math.floor(((tc + 1) * cropWidth) / targetCols);

				let picked = ' ';
				for (let sr = sourceRowStart; sr < Math.max(sourceRowStart + 1, sourceRowEndExclusive); sr++) {
					const line = rows[minRow + sr] ?? '';
					for (
						let sc = sourceColStart;
						sc < Math.max(sourceColStart + 1, sourceColEndExclusive);
						sc++
					) {
						const symbol = line[minCol + sc] ?? ' ';
						if (isPreviewFilledCell(symbol)) {
							picked = symbol;
							break;
						}
					}
					if (picked !== ' ') break;
				}

				preview[tr][tc] = picked;
			}
		}

		return preview;
	}

	function getSyntaxSymbols(syntaxes: Array<Syntax>): Array<string> {
		const symbols = new Set<string>();
		for (const syntax of syntaxes) {
			for (const rule of syntax.rules) {
				if (rule.symbol?.length === 1) {
					symbols.add(rule.symbol);
				}
			}
		}
		return Array.from(symbols);
	}

	function getRulesBySymbol(syntaxes: Array<Syntax>): Record<string, Rule> {
		const map: Record<string, Rule> = {};
		for (const syntax of syntaxes) {
			for (const rule of syntax.rules) {
				if (rule.symbol?.length === 1 && !map[rule.symbol]) {
					map[rule.symbol] = rule;
				}
			}
		}
		return map;
	}

	function getVoidFillSymbol(rules: Record<string, Rule>): string {
		// Prefer a visible symbol mapped as Void (not blank space).
		for (const [symbol, rule] of Object.entries(rules)) {
			if (rule.shape.kind === ShapeKind.Void && symbol !== ' ') {
				return symbol;
			}
		}

		// If only blank-space Void exists, use a visible fallback.
		return '.';
	}

	function fillVoidInStructure(
		structure: string,
		fillSymbol: string,
		targetHeight: number,
		occupiedStructure: string
	): string {
		const parsed = parseGlyphStructure(structure);
		const sourceRows = parsed.body ? parsed.body.split(/\r?\n/) : [];
		const occupiedRows = occupiedStructure ? occupiedStructure.split(/\r?\n/) : [];
		const width = Math.max(
			1,
			...sourceRows.map((row) => row.length),
			...occupiedRows.map((row) => row.length)
		);
		const height = Math.max(1, targetHeight, sourceRows.length, occupiedRows.length);

		if (width <= 0) return structure;

		const nextBody = Array.from({ length: height }, (_, rowIndex) => {
			const sourceRow = (sourceRows[rowIndex] ?? '').padEnd(width, ' ');
			const occupiedRow = (occupiedRows[rowIndex] ?? '').padEnd(width, ' ');
			return Array.from({ length: width }, (_, colIndex) => {
				const sourceCell = sourceRow[colIndex] ?? ' ';
				if (sourceCell !== ' ') return sourceCell;
				const occupiedCell = occupiedRow[colIndex] ?? ' ';
				return occupiedCell === ' ' ? fillSymbol : ' ';
			}).join('');
		})
			.join('\n');

		return replaceGlyphStructureBody(structure, nextBody);
	}

	function getGlyphComponents(glyph: GlyphInput): Array<GlyphComponentRef> {
		return parseGlyphStructure(glyph.structure).components;
	}

	function getGlyphBody(glyph: GlyphInput): string {
		return parseGlyphStructure(glyph.structure).body;
	}

	function getResolvedGlyphBody(glyph: GlyphInput): string {
		return resolvedGlyphStructuresVisualDataForDisplay.get(glyph.name)?.body ?? getGlyphBody(glyph);
	}

	function getResolvedGlyphComponentSources(glyph: GlyphInput): Array<Array<Array<string>>> {
		return resolvedGlyphStructuresVisualDataForDisplay.get(glyph.name)?.componentSources ?? [];
	}

	function structureHasDesignMarks(structure: string): boolean {
		for (const char of Array.from(structure.replace(/\n/g, ''))) {
			if (char !== ' ' && char !== '.') return true;
		}
		return false;
	}

	function isGlyphDesigned(glyph: GlyphInput): boolean {
		const resolved = resolvedGlyphStructuresVisualDataForDisplay.get(glyph.name)?.body ?? getGlyphBody(glyph);
		return structureHasDesignMarks(resolved);
	}

	function getGlyphSetID(glyph: GlyphInput) {
		return inferGlyphSetID(glyph);
	}

	function getGlyphStructureTextareaValue(glyph: GlyphInput): string {
		return parseGlyphStructure(glyph.structure).body;
	}

	function getGlyphStructureComponentSymbolViewValue(glyph: GlyphInput): string {
		const parsed = parseGlyphStructure(glyph.structure);
		return serializeGlyphStructure({
			components: parsed.components,
			body: resolvedGlyphStructuresComponentSymbolTextForDisplay.get(glyph.name) ?? parsed.body
		});
	}

	function handleGlyphStructureInput(glyph: GlyphInput, nextValue: string) {
		glyph.structure = replaceGlyphStructureBody(glyph.structure, nextValue);
		scheduleTouchGlyphs();
	}

	let touchGlyphsTimer: ReturnType<typeof setTimeout> | undefined;

	function touchGlyphs() {
		$glyphs = [...$glyphs];
	}

	function scheduleTouchGlyphs() {
		if (touchGlyphsTimer) return;
		touchGlyphsTimer = setTimeout(() => {
			touchGlyphsTimer = undefined;
			touchGlyphs();
		}, 16);
	}

	function inputValue(event: Event): string {
		const input = (event.target || event.currentTarget) as
			| HTMLInputElement
			| HTMLTextAreaElement
			| null;
		return input?.value ?? '';
	}

	function getAvailableComponentGlyphs(targetGlyphName: string): Array<GlyphInput> {
		return sortGlyphs(
			$glyphs.filter(
				(glyph) => isComponentGlyphName(glyph.name) && glyph.name !== targetGlyphName
			)
		);
	}

	function normalizeComponentSymbolInput(value: string): string {
		return Array.from((value ?? '').trim())[0] ?? '';
	}

	type ParsedComponentVariantName = {
		stem: string;
		stylisticSetIndex: number | null;
	};

	function parseComponentVariantName(name: string): ParsedComponentVariantName | undefined {
		const trimmed = (name ?? '').trim();
		const match = trimmed.match(/^(.*?)(?:\.ss(\d\d))?\.component$/i);
		if (!match) return undefined;
		const stem = (match[1] ?? '').trim();
		if (!stem) return undefined;
		const stylisticSetIndex = match[2] ? Number.parseInt(match[2], 10) : null;
		return {
			stem,
			stylisticSetIndex: Number.isFinite(stylisticSetIndex) ? stylisticSetIndex : null
		};
	}

	function getComponentVariantNames(componentName: string): Array<string> {
		const parsed = parseComponentVariantName(componentName);
		if (!parsed) return [componentName];
		const stemLower = parsed.stem.toLowerCase();
		const variants = new Set<string>([componentName]);
		for (const glyph of $glyphs) {
			if (!isComponentGlyphName(glyph.name)) continue;
			const candidate = parseComponentVariantName(glyph.name);
			if (!candidate) continue;
			if (candidate.stem.toLowerCase() !== stemLower) continue;
			variants.add(glyph.name);
		}

		return Array.from(variants).sort((a, b) => {
			const parsedA = parseComponentVariantName(a);
			const parsedB = parseComponentVariantName(b);
			const orderA = parsedA?.stylisticSetIndex ?? 0;
			const orderB = parsedB?.stylisticSetIndex ?? 0;
			if (orderA !== orderB) return orderA - orderB;
			return a.localeCompare(b);
		});
	}

	function getCurrentComponentVariantLabel(componentName: string): string {
		const parsed = parseComponentVariantName(componentName);
		if (!parsed) return componentName;
		if (parsed.stylisticSetIndex === null) return 'base';
		return `ss${String(parsed.stylisticSetIndex).padStart(2, '0')}`;
	}

	function getNextComponentVariantName(componentName: string): string {
		const variants = getComponentVariantNames(componentName);
		if (variants.length <= 1) return componentName;
		const currentIndex = variants.indexOf(componentName);
		const safeIndex = currentIndex >= 0 ? currentIndex : 0;
		return variants[(safeIndex + 1) % variants.length];
	}

	function getComponentSymbolCandidates(componentName: string): Array<string> {
		const withoutComponentSuffix = componentName.replace(/\.component$/i, '');
		const seen = new Set<string>();
		const candidates: Array<string> = [];

		for (const char of Array.from(withoutComponentSuffix)) {
			if (!/[A-Za-z0-9]/.test(char)) continue;
			if (seen.has(char)) continue;
			seen.add(char);
			candidates.push(char);
		}

		return candidates;
	}

	function pickAutoComponentSymbol(componentName: string, takenSymbols: Set<string>): string {
		for (const candidate of getComponentSymbolCandidates(componentName)) {
			if (!takenSymbols.has(candidate)) return candidate;
		}

		const fallbackSymbols = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
		for (const candidate of Array.from(fallbackSymbols)) {
			if (!takenSymbols.has(candidate)) return candidate;
		}

		return '';
	}

	function normalizeComponentRotationInput(value: number): number {
		if (!Number.isFinite(value)) return 0;
		const stepped = Math.round(value / 15) * 15;
		const normalized = ((stepped % 360) + 360) % 360;
		return normalized === 360 ? 0 : normalized;
	}

	function normalizeComponentPositionInput(value: number): number {
		if (!Number.isFinite(value)) return 1;
		return Math.trunc(value);
	}

	function parseComponentPositionInput(value: string, fallback: number): number {
		const parsed = Number.parseInt(value.trim(), 10);
		if (!Number.isFinite(parsed)) return normalizeComponentPositionInput(fallback);
		return normalizeComponentPositionInput(parsed);
	}

	function addComponentReference(targetGlyph: GlyphInput) {
		if (!newComponentName || newComponentName === targetGlyph.name) return;
		const existingComponents = getGlyphComponents(targetGlyph);
		const takenSymbols = new Set(
			existingComponents
				.map((component) => normalizeComponentSymbolInput(component.symbol))
				.filter(Boolean)
		);

		const component: GlyphComponentRef = {
			name: newComponentName,
			symbol: pickAutoComponentSymbol(newComponentName, takenSymbols),
			x: 1,
			y: 1,
			rotation: 0
		};

		targetGlyph.structure = replaceGlyphStructureComponents(targetGlyph.structure, [
			...existingComponents,
			component
		]);
		touchGlyphs();
	}

	function removeComponentReference(targetGlyph: GlyphInput, componentIndex: number) {
		const currentComponents = getGlyphComponents(targetGlyph);
		targetGlyph.structure = replaceGlyphStructureComponents(
			targetGlyph.structure,
			currentComponents.filter((_, index) => index !== componentIndex)
		);
		touchGlyphs();
	}

	function updateComponentReference(
		targetGlyph: GlyphInput,
		componentIndex: number,
		updater: (component: GlyphComponentRef) => GlyphComponentRef
	) {
		const currentComponents = getGlyphComponents(targetGlyph);
		if (componentIndex < 0 || componentIndex >= currentComponents.length) return;
		targetGlyph.structure = replaceGlyphStructureComponents(
			targetGlyph.structure,
			currentComponents.map((component, index) =>
				index === componentIndex ? updater(component) : component
			)
		);
		touchGlyphs();
	}

	function cycleComponentReferenceVariant(targetGlyph: GlyphInput, componentIndex: number) {
		updateComponentReference(targetGlyph, componentIndex, (existing) => ({
			...existing,
			name: getNextComponentVariantName(existing.name)
		}));
	}

	type GlyphEditorTab = 'visualDesign' | 'glyphStructure';

	let isAddGlyphModalOpen = false;
	let isAddGlyphSetModalOpen = false;
	let activeGlyphEditorTab: GlyphEditorTab = 'visualDesign';
	let isZenMode = false;
	let isDesignFullscreen = false;
	let designWorkspaceElement: HTMLDivElement | undefined;
	let floatingToolbarElement: HTMLDivElement | undefined;
	let floatingToolbarX = 24;
	let floatingToolbarY = 88;
	let isFloatingToolbarDragging = false;
	let floatingToolbarDragOffsetX = 0;
	let floatingToolbarDragOffsetY = 0;
	const previewCanvasHeight = 500;
	let newComponentName = '';
	let selectedGlyphSetFilter = 'all';
	let showOnlyUndesignedGlyphs = false;
	let selectedBrushSymbol = '';
	const glyphBodyRotationProgressByID = new Map<string, number>();

	$: brushSymbols = getSyntaxSymbols($syntaxes);
	$: rulesBySymbol = getRulesBySymbol($syntaxes);
	$: voidFillSymbol = getVoidFillSymbol(rulesBySymbol);
	$: fillTargetHeight = Math.max(1, Math.round($metrics.height || 1));
	$: glyphSetDefinitions = getGlyphSetDefinitions();
	$: filteredGlyphs = sortGlyphs($glyphs).filter((glyph) => {
		if (selectedGlyphSetFilter !== 'all' && getGlyphSetID(glyph) !== selectedGlyphSetFilter) {
			return false;
		}
		if (showOnlyUndesignedGlyphs && isGlyphDesigned(glyph)) {
			return false;
		}
		return true;
	});
	$: if (filteredGlyphs.length && !filteredGlyphs.some((glyph) => glyph.id === $selectedGlyph)) {
		$selectedGlyph = filteredGlyphs[0].id;
	}
	$: selectedGlyphData = $glyphs.find((glyph) => glyph.id === $selectedGlyph);
	$: selectableComponentGlyphNames = selectedGlyphData
		? getAvailableComponentGlyphs(selectedGlyphData.name).map((glyph) => glyph.name)
		: [];
	$: if (!selectableComponentGlyphNames.includes(newComponentName)) {
		newComponentName = selectableComponentGlyphNames[0] ?? '';
	}
	function isTypingTarget(target: EventTarget | null): boolean {
		if (!(target instanceof HTMLElement)) return false;
		const tagName = target.tagName.toLowerCase();
		if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') return true;
		return target.isContentEditable;
	}

	function resolveBrushShortcutSymbol(key: string, symbols: Array<string>): string | undefined {
		if (key.length !== 1) return undefined;
		if (symbols.includes(key)) return key;

		const keyLower = key.toLowerCase();
		return symbols.find((symbol) => symbol.length === 1 && symbol.toLowerCase() === keyLower);
	}

	function setZenMode(nextValue: boolean) {
		isZenMode = nextValue;
		if (isZenMode) {
			activeGlyphEditorTab = 'visualDesign';
		}
	}

	function toggleZenMode() {
		setZenMode(!isZenMode);
	}

	function clearSelectedGlyphDesign() {
		if (!selectedGlyphData) return;
		selectedGlyphData.structure = replaceGlyphStructureBody(selectedGlyphData.structure, '');
		touchGlyphs();
	}

	function fillVoidSelectedGlyph() {
		if (!selectedGlyphData) return;
		selectedGlyphData.structure = fillVoidInStructure(
			selectedGlyphData.structure,
			voidFillSymbol,
			fillTargetHeight,
			getResolvedGlyphBody(selectedGlyphData)
		);
		touchGlyphs();
	}

	function mirrorSelectedGlyphLeftRight() {
		if (!selectedGlyphData) return;
		const parsed = parseGlyphStructure(selectedGlyphData.structure);
		selectedGlyphData.structure = replaceGlyphStructureBody(
			selectedGlyphData.structure,
			reflectGlyphStructureBody(parsed.body, 'vertical', rulesBySymbol)
		);
		touchGlyphs();
	}

	function flipSelectedGlyphTopBottom() {
		if (!selectedGlyphData) return;
		const parsed = parseGlyphStructure(selectedGlyphData.structure);
		selectedGlyphData.structure = replaceGlyphStructureBody(
			selectedGlyphData.structure,
			reflectGlyphStructureBody(parsed.body, 'horizontal', rulesBySymbol)
		);
		touchGlyphs();
	}

	function remapDirectionalSymbolsByQuarterTurns(body: string, quarterTurns: number): string {
		if (!quarterTurns) return body;
		const mappedSymbols = mapDirectionalSymbolsForRotation(rulesBySymbol, quarterTurns * 90);
		if (!mappedSymbols.size) return body;

		return body
			.split('\n')
			.map((row) => replaceMappedSymbols(row, mappedSymbols))
			.join('\n');
	}

	function rotateSelectedGlyphClockwise() {
		if (!selectedGlyphData) return;
		const parsed = parseGlyphStructure(selectedGlyphData.structure);
		const rotationStep = 15;
		const previousRotationProgress = glyphBodyRotationProgressByID.get(selectedGlyphData.id) ?? 0;
		const crossedQuarterTurns = getCrossedQuarterTurns(previousRotationProgress, rotationStep);
		let rotatedBody = rotateGlyphStructureBody(parsed.body, rotationStep, {});
		rotatedBody = remapDirectionalSymbolsByQuarterTurns(rotatedBody, crossedQuarterTurns);
		const rotatedComponents = parsed.components.map((component) => ({
			...component,
			rotation: normalizeComponentRotationInput(component.rotation + rotationStep)
		}));

		let nextStructure = replaceGlyphStructureBody(selectedGlyphData.structure, rotatedBody);
		if (parsed.components.length) {
			nextStructure = replaceGlyphStructureComponents(nextStructure, rotatedComponents);
		}
		selectedGlyphData.structure = nextStructure;
		glyphBodyRotationProgressByID.set(selectedGlyphData.id, previousRotationProgress + rotationStep);
		touchGlyphs();
	}

	function getFloatingToolbarSize(): { width: number; height: number } {
		if (!floatingToolbarElement) {
			return { width: 160, height: 220 };
		}
		const rect = floatingToolbarElement.getBoundingClientRect();
		return {
			width: Math.max(80, Math.round(rect.width)),
			height: Math.max(80, Math.round(rect.height))
		};
	}

	function clampFloatingToolbarPosition(x: number, y: number): { x: number; y: number } {
		if (typeof window === 'undefined') {
			return { x, y };
		}
		const margin = 8;
		const { width, height } = getFloatingToolbarSize();
		const maxX = Math.max(margin, window.innerWidth - width - margin);
		const maxY = Math.max(margin, window.innerHeight - height - margin);
		return {
			x: Math.min(Math.max(margin, Math.round(x)), maxX),
			y: Math.min(Math.max(margin, Math.round(y)), maxY)
		};
	}

	function setFloatingToolbarPosition(x: number, y: number) {
		const next = clampFloatingToolbarPosition(x, y);
		floatingToolbarX = next.x;
		floatingToolbarY = next.y;
	}

	function positionFloatingToolbarDefault() {
		if (typeof window === 'undefined') return;
		const { width } = getFloatingToolbarSize();
		setFloatingToolbarPosition(window.innerWidth - width - 16, 96);
	}

	function startFloatingToolbarDrag(event: PointerEvent) {
		if (!(event.currentTarget instanceof HTMLElement)) return;
		event.preventDefault();
		isFloatingToolbarDragging = true;
		floatingToolbarDragOffsetX = event.clientX - floatingToolbarX;
		floatingToolbarDragOffsetY = event.clientY - floatingToolbarY;
		event.currentTarget.setPointerCapture(event.pointerId);
	}

	function onFloatingToolbarPointerMove(event: PointerEvent) {
		if (!isFloatingToolbarDragging) return;
		setFloatingToolbarPosition(
			event.clientX - floatingToolbarDragOffsetX,
			event.clientY - floatingToolbarDragOffsetY
		);
	}

	function stopFloatingToolbarDrag() {
		isFloatingToolbarDragging = false;
	}

	function onFloatingToolbarResize() {
		setFloatingToolbarPosition(floatingToolbarX, floatingToolbarY);
	}

	function syncDesignFullscreenState() {
		if (typeof document === 'undefined') return;
		isDesignFullscreen = document.fullscreenElement === designWorkspaceElement;
	}

	async function toggleDesignFullscreen() {
		if (!designWorkspaceElement || typeof document === 'undefined') return;
		try {
			if (document.fullscreenElement === designWorkspaceElement) {
				await document.exitFullscreen();
				return;
			}
			await designWorkspaceElement.requestFullscreen();
		} catch (error) {
			console.error('Unable to toggle design fullscreen', error);
		}
	}

	onMount(() => {
		if (typeof document === 'undefined') return;

		const onFullscreenChange = () => {
			syncDesignFullscreenState();
		};

			const onKeyDown = (event: KeyboardEvent) => {
				if (isTypingTarget(event.target)) return;
				if (event.repeat) return;
				if (event.defaultPrevented) return;

				const key = event.key;
				const plain = !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey;
				const normalizedKey = key.toLowerCase();
				if (plain && normalizedKey === 'r') {
					event.preventDefault();
					rotateSelectedGlyphClockwise();
					return;
				}

				const symbolShortcutAllowed = !event.ctrlKey && !event.metaKey && !event.altKey;
				const shortcutBrushSymbol = symbolShortcutAllowed
					? resolveBrushShortcutSymbol(key, brushSymbols)
					: undefined;
				if (shortcutBrushSymbol) {
					event.preventDefault();
					selectedBrushSymbol = shortcutBrushSymbol;
					return;
				}

				if (plain && normalizedKey === 'z') {
					event.preventDefault();
					toggleZenMode();
					return;
				}

				if (plain && normalizedKey === 'f') {
					event.preventDefault();
					void toggleDesignFullscreen();
					return;
				}

				if (plain && normalizedKey === 'v') {
					event.preventDefault();
					fillVoidSelectedGlyph();
					return;
				}

				if (plain && normalizedKey === 'm') {
					event.preventDefault();
					mirrorSelectedGlyphLeftRight();
					return;
				}

				if (plain && normalizedKey === 'x') {
					event.preventDefault();
					flipSelectedGlyphTopBottom();
					return;
				}

				if (plain && normalizedKey === 'c') {
					event.preventDefault();
					clearSelectedGlyphDesign();
					return;
				}

				if (normalizedKey === 'escape' && isZenMode) {
					event.preventDefault();
					setZenMode(false);
				}
		};

		document.addEventListener('fullscreenchange', onFullscreenChange);
		window.addEventListener('keydown', onKeyDown);
		syncDesignFullscreenState();
		requestAnimationFrame(() => {
			positionFloatingToolbarDefault();
		});

		return () => {
			document.removeEventListener('fullscreenchange', onFullscreenChange);
			window.removeEventListener('keydown', onKeyDown);
		};
	});
</script>

<!--  -->

<svelte:window
	on:pointermove={onFloatingToolbarPointerMove}
	on:pointerup={stopFloatingToolbarDrag}
	on:pointercancel={stopFloatingToolbarDrag}
	on:resize={onFloatingToolbarResize}
/>

<div class="flex flex-row flex-nowrap items-stretch overflow-hidden grow">
	{#if !isZenMode}
		<div class="shrink-0 flex items-stretch">
			<Sidebar>
				<svelte:fragment slot="topArea">
					<div class="space-y-2">
						<div class="flex gap-2">
							<Button
								on:click={() => {
									isAddGlyphModalOpen = true;
								}}>+ Aggiungi glifo</Button
							>
							<Button
								on:click={() => {
									isAddGlyphSetModalOpen = true;
								}}>+ Aggiungi set</Button
							>
						</div>

						<div class="space-y-1 font-mono text-xs">
							<p class="text-slate-600">Filtro set</p>
							<select class="w-full h-9 bg-slate-200 px-2" bind:value={selectedGlyphSetFilter}>
								<option value="all">Tutti i set</option>
								{#each glyphSetDefinitions as definition (definition.id)}
									<option value={definition.id}>{definition.label}</option>
								{/each}
							</select>
						</div>

						<label class="flex items-center gap-2 font-mono text-xs text-slate-700">
							<input type="checkbox" bind:checked={showOnlyUndesignedGlyphs} />
							<span>Mostra solo glifi vuoti</span>
						</label>
					</div>
				</svelte:fragment>
				<svelte:fragment slot="listTitle">Lista glifi</svelte:fragment>
				<svelte:fragment slot="items">
					{#if filteredGlyphs.length === 0}
						<p class="font-mono text-xs text-slate-500 px-2 py-1">
							Nessun glifo nel filtro corrente.
						</p>
					{/if}
					{#each filteredGlyphs as g (g.id)}
						{@const glyphPreviewSymbols = getMiniGlyphPreviewSymbols(getResolvedGlyphBody(g))}
						{@const designed = isGlyphDesigned(g)}
						<SidebarTile selection={selectedGlyph} id={g.id}>
							<div class="flex items-start justify-between gap-2">
								<div class="min-w-0 flex items-start gap-2">
									<div class="h-8 w-8 shrink-0 border border-slate-400 bg-white p-[1px]">
										<div
											class="grid h-full w-full gap-[1px] bg-slate-300"
											style={`grid-template-columns: repeat(${glyphPreviewSymbols[0]?.length ?? 1}, minmax(0, 1fr)); grid-template-rows: repeat(${glyphPreviewSymbols.length || 1}, minmax(0, 1fr));`}
										>
											{#each glyphPreviewSymbols as previewRow}
												{#each previewRow as previewSymbol}
													{@const previewRule = rulesBySymbol[previewSymbol]}
													<div class="relative overflow-hidden bg-white text-slate-900">
														{#if previewRule && previewRule.shape.kind !== ShapeKind.Void}
															<RuleShapePreview rule={previewRule} className="h-full w-full" />
														{:else if isPreviewFilledCell(previewSymbol)}
															<span class="block h-full w-full bg-slate-900"></span>
														{/if}
													</div>
												{/each}
											{/each}
										</div>
									</div>

									<div class="min-w-0">
										<div class="truncate opacity-90">{g.name}</div>
									</div>
								</div>
								<span class={designed ? 'text-emerald-500' : 'text-rose-500'}>
									{designed ? '●' : '○'}
								</span>
							</div>
						</SidebarTile>
					{/each}
				</svelte:fragment>
			</Sidebar>
		</div>
	{/if}

	<!-- Glyph area -->
	<div class={`grow flex flex-col items-stretch ${isZenMode ? 'p-2 space-y-2' : 'p-8 space-y-8'}`}>
		{#each $glyphs as g}
			{#if g.id == $selectedGlyph}
				{@const designed = isGlyphDesigned(g)}
				{@const glyphString = getGlyphString(g.name)}
				{@const glyphHex = getGlyphUnicodeHex(g.name)}
				{@const glyphSet = getGlyphSetByID(getGlyphSetID(g))}
				{@const componentGlyphs = getAvailableComponentGlyphs(g.name)}
				{@const glyphComponents = getGlyphComponents(g)}
				{@const resolvedGlyphBody = getResolvedGlyphBody(g)}
				{@const resolvedGlyphComponentSources = getResolvedGlyphComponentSources(g)}
					{@const glyphStructureValue = getGlyphStructureTextareaValue(g)}
					{@const glyphStructureComponentSymbolViewValue = getGlyphStructureComponentSymbolViewValue(g)}
					{@const glyphStructureLines = glyphStructureValue ? glyphStructureValue.split(/\r?\n/) : ['']}
				{@const glyphStructureLineCount = Math.max(1, glyphStructureLines.length)}
				{@const glyphStructureColumnCount = Math.max(
					1,
					...glyphStructureLines.map((line) => line.length)
				)}
				{#if !isZenMode}
					<div class="shrink-0 flex justify-between items-center gap-4">
						<div class="min-w-0 flex items-center gap-3">
							<div class="flex items-center gap-2 text-lg">
								<span class={designed ? 'text-emerald-500' : 'text-rose-500'}>
									{designed ? '●' : '○'}
								</span>
								<p class="text-slate-700">{g.name}</p>
							</div>
							{#if glyphString}
								<span class="text-slate-600">{glyphString}</span>
							{/if}
							{#if glyphHex}
								<span class="text-sm text-slate-500">U+{glyphHex}</span>
							{/if}
							<span class="text-sm text-slate-500">[{glyphSet.label}]</span>
						</div>
						<DeleteButton on:delete={handleDelete} />
					</div>
					<hr />
				{/if}
				<div
					bind:this={designWorkspaceElement}
					class={`h-0 grow min-h-0 flex gap-4 ${isZenMode ? 'flex-row' : 'flex-col lg:flex-row'}`}
				>
					<div class="min-h-0 min-w-0 flex-1 flex flex-col gap-2">
						<div class="h-0 grow min-h-0 flex flex-col">
							<div class="mb-2 flex items-center gap-2 border-b border-slate-300 pb-2">
								<button
									type="button"
									class={`px-3 py-2 text-sm font-mono ${
										activeGlyphEditorTab === 'visualDesign'
											? 'bg-slate-800 text-white'
											: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
									}`}
									aria-pressed={activeGlyphEditorTab === 'visualDesign'}
									on:click={() => {
										activeGlyphEditorTab = 'visualDesign';
									}}
								>
									Visual design
								</button>
								<button
									type="button"
									class={`px-3 py-2 text-sm font-mono ${
										activeGlyphEditorTab === 'glyphStructure'
											? 'bg-slate-800 text-white'
											: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
									}`}
									aria-pressed={activeGlyphEditorTab === 'glyphStructure'}
									on:click={() => {
										activeGlyphEditorTab = 'glyphStructure';
									}}
									>
										Glyph structure
									</button>
								</div>
				
								<div
									class="mb-2 p-2 border border-slate-300 bg-slate-50 space-y-2 font-mono text-xs"
								>
									<p class="text-slate-900">
										Componenti ({glyphComponents.length})
									</p>
				
										{#if componentGlyphs.length}
											<div class="flex flex-wrap items-end gap-2">
												<div class="flex flex-col gap-1">
													<label class="text-slate-500" for="component-name-select">Nome</label>
												<select
													id="component-name-select"
													class="h-10 bg-slate-200 px-2"
													bind:value={newComponentName}
												>
														{#each componentGlyphs as componentGlyph (componentGlyph.id)}
															<option value={componentGlyph.name}>{componentGlyph.name}</option>
														{/each}
													</select>
												</div>
												<Button on:click={() => addComponentReference(g)}
													>+ Aggiungi componente</Button
												>
											</div>
									{:else}
										<p class="text-slate-500">
											Nessun glifo `.component` disponibile. Crea un glifo come
											`etom.component`.
										</p>
									{/if}
				
										{#if glyphComponents.length}
											<div class="space-y-1">
												{#each glyphComponents as component, index (`${component.name}:${index}`)}
													{@const componentVariantNames = getComponentVariantNames(component.name)}
													<div
														class="flex flex-wrap items-center gap-2 border border-slate-200 bg-white px-2 py-1"
													>
														<p class="max-w-[260px] truncate text-[11px]">{component.name}</p>
														<button
															type="button"
															class="h-6 px-2 bg-slate-200 text-[11px] text-slate-800 hover:bg-slate-300 disabled:opacity-40 disabled:hover:bg-slate-200"
															disabled={componentVariantNames.length <= 1}
															title={
																componentVariantNames.length > 1
																	? `Varianti: ${componentVariantNames.join(', ')}`
																	: 'Nessuna variante ss disponibile'
															}
															on:click={() => cycleComponentReferenceVariant(g, index)}
														>
															ss {getCurrentComponentVariantLabel(component.name)} ↻
														</button>
														<div class="flex items-center gap-1">
															<span class="text-[10px] text-slate-500">x</span>
															<input
																type="number"
																step="1"
																class="h-6 w-14 bg-slate-100 px-1 text-[11px]"
																value={normalizeComponentPositionInput(component.x)}
																on:change={(event) =>
																	updateComponentReference(g, index, (existing) => ({
																		...existing,
																		x: parseComponentPositionInput(
																			inputValue(event),
																			existing.x
																		)
																	}))}
															/>
														</div>
														<div class="flex items-center gap-1">
															<span class="text-[10px] text-slate-500">y</span>
															<input
																type="number"
																step="1"
																class="h-6 w-14 bg-slate-100 px-1 text-[11px]"
																value={normalizeComponentPositionInput(component.y)}
																on:change={(event) =>
																	updateComponentReference(g, index, (existing) => ({
																		...existing,
																		y: parseComponentPositionInput(
																			inputValue(event),
																			existing.y
																		)
																	}))}
															/>
														</div>
														<button
															type="button"
															class={`h-6 px-2 text-[11px] ${
																component.flipped
																	? 'bg-slate-800 text-white'
																	: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
															}`}
															on:click={() =>
																updateComponentReference(g, index, (existing) => ({
																	...existing,
																	flipped: !existing.flipped
																}))}
														>
															flipped
														</button>
														<button
															type="button"
															class={`h-6 px-2 text-[11px] ${
																component.mirrored
																	? 'bg-slate-800 text-white'
																	: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
															}`}
															on:click={() =>
																updateComponentReference(g, index, (existing) => ({
																	...existing,
																	mirrored: !existing.mirrored
																}))}
														>
															mirrored
														</button>
														<button
															type="button"
															class="h-6 px-2 bg-slate-200 text-[11px] text-slate-800 hover:bg-slate-300"
															on:click={() =>
																updateComponentReference(g, index, (existing) => ({
																	...existing,
																	rotation: normalizeComponentRotationInput(existing.rotation + 15)
																}))}
														>
															rotate +15°
														</button>
														<button
															type="button"
															class="h-6 px-2 text-[11px] text-red-600 hover:text-red-800"
															on:click={() => removeComponentReference(g, index)}
														>
															rimuovi
														</button>
													</div>
												{/each}
											</div>
									{/if}
								</div>
			
								<div class="h-0 grow min-h-0 flex flex-col gap-2">
									<div
										class={activeGlyphEditorTab === 'visualDesign'
											? 'h-0 grow min-h-0'
											: 'shrink-0'}
									>
												<GlyphPainter
													bind:structure={g.structure}
													bind:selectedBrush={selectedBrushSymbol}
													resolvedBody={resolvedGlyphBody}
													resolvedComponentSources={resolvedGlyphComponentSources}
													brushes={brushSymbols}
											{rulesBySymbol}
											showGrid={activeGlyphEditorTab === 'visualDesign'}
											on:change={scheduleTouchGlyphs}
										/>
									</div>
			
											{#if activeGlyphEditorTab === 'glyphStructure'}
												<div
													class={`grow min-h-0 h-full flex flex-col gap-2 ${
														glyphComponents.length ? 'xl:flex-row' : ''
													}`}
												>
													<div class="grow min-h-0 h-full flex flex-col gap-1 xl:flex-1 xl:basis-0">
														<div class="shrink-0 flex items-center justify-between font-mono text-xs text-slate-500">
															<span>rows: 1..{glyphStructureLineCount}</span>
															<span>cols: 1..{glyphStructureColumnCount}</span>
														</div>
														<textarea
															class="grow min-h-0 h-full p-2 bg-slate-200 hover:bg-slate-300 font-mono text-xl focus:ring-4"
															value={glyphStructureValue}
															wrap="off"
															spellcheck="false"
															on:input={(event) => {
																handleGlyphStructureInput(g, inputValue(event));
														}}
													/>
												</div>

													{#if glyphComponents.length}
														<div class="grow min-h-0 h-full flex flex-col gap-1 xl:flex-1 xl:basis-0">
															<div class="shrink-0 flex items-center justify-between font-mono text-xs text-slate-500">
																<span>Component symbol map</span>
																<span>read-only</span>
															</div>
															<textarea
																class="grow min-h-0 h-full p-2 bg-slate-100 font-mono text-xl text-slate-700"
																value={glyphStructureComponentSymbolViewValue}
																wrap="off"
																spellcheck="false"
																readonly
															/>
													</div>
												{/if}
											</div>
										{/if}
								</div>
							</div>
						</div>
			
						<div
							class={`min-h-0 min-w-0 flex flex-col bg-slate-50 ${
								isZenMode
									? 'flex-[0_0_42%] border-l border-slate-300 pl-2'
									: 'flex-1 lg:border-l lg:border-slate-300 lg:pl-4'
							}`}
						>
							<p class="text-small font-mono text-slate-900 mb-2 text-sm">
								Anteprima e metriche
							</p>
							<div class="h-0 grow min-h-0 overflow-y-auto overflow-x-hidden">
								<GlyphPreview
									canvasHeight={previewCanvasHeight}
									showTitle={false}
									showLegend={false}
								/>
							</div>
						</div>
					</div>
				{/if}
			{/each}
	</div>
</div>

<div
	bind:this={floatingToolbarElement}
	class={`fixed z-50 w-44 border border-slate-300 bg-white shadow-lg ${isFloatingToolbarDragging ? 'cursor-grabbing' : ''}`}
	style={`left: ${floatingToolbarX}px; top: ${floatingToolbarY}px;`}
>
	<button
		type="button"
		class="h-8 px-2 border-b border-slate-300 bg-slate-900 text-white font-mono text-xs flex items-center justify-between cursor-grab select-none"
		title="Trascina toolbar"
		on:pointerdown={startFloatingToolbarDrag}
	>
		<span>Tools</span>
		<span>::</span>
	</button>
	<div class="p-2 flex flex-col gap-1 bg-white">
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2"
			title="Zen mode (Z)"
			on:click={toggleZenMode}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>Z</span
			>
			<span>Zen mode</span>
			<span class="ml-auto text-[10px] text-slate-500">Z</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2"
			title="Full screen (F)"
			on:click={toggleDesignFullscreen}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>F</span
			>
			<span>{isDesignFullscreen ? 'Exit full screen' : 'Full screen'}</span>
			<span class="ml-auto text-[10px] text-slate-500">F</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Mirror L/R (M)"
			on:click={mirrorSelectedGlyphLeftRight}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>M</span
			>
			<span>Mirror L/R</span>
			<span class="ml-auto text-[10px] text-slate-500">M</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Flip U/D (X)"
			on:click={flipSelectedGlyphTopBottom}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>X</span
			>
			<span>Flip U/D</span>
			<span class="ml-auto text-[10px] text-slate-500">X</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Rotate 15° (R)"
			on:click={rotateSelectedGlyphClockwise}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>R</span
			>
			<span>Rotate 15°</span>
			<span class="ml-auto text-[10px] text-slate-500">R</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Fill void (V)"
			on:click={fillVoidSelectedGlyph}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>V</span
			>
			<span>Fill void</span>
			<span class="ml-auto text-[10px] text-slate-500">V</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Pulisci (C)"
			on:click={clearSelectedGlyphDesign}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>C</span
			>
			<span>Pulisci</span>
			<span class="ml-auto text-[10px] text-slate-500">C</span>
		</button>
	</div>
</div>

<!--  -->

<AddGlyphModal bind:open={isAddGlyphModalOpen} />
<AddGlyphSetModal bind:open={isAddGlyphSetModalOpen} />
