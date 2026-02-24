<script lang="ts">
	import { Orientation, ShapeKind, ValueKind, type Rule } from '$lib/types';

	export let rule: Rule | undefined;
	export let className = '';

	type Point = {
		x: number;
		y: number;
	};

	function clamp(value: number, min: number, max: number): number {
		return Math.max(min, Math.min(max, value));
	}

	function readNumberProp(prop: any, fallback: number): number {
		if (!prop || !prop.value) return fallback;
		if (prop.value.kind === ValueKind.Fixed) return prop.value.data;
		if (prop.value.kind === ValueKind.Choice) return prop.value.data.options?.[0] ?? fallback;
		if (prop.value.kind === ValueKind.Range) {
			const { min = fallback, max = fallback } = prop.value.data ?? {};
			return (min + max) / 2;
		}
		return fallback;
	}

	function readBooleanProp(prop: any, fallback: boolean): boolean {
		if (!prop || !prop.value) return fallback;
		if (prop.value.kind === ValueKind.Fixed) return prop.value.data;
		if (prop.value.kind === ValueKind.Choice) return prop.value.data.options?.[0] ?? fallback;
		return fallback;
	}

	function readOrientationProp(prop: any, fallback: Orientation): Orientation {
		if (!prop || !prop.value) return fallback;
		if (prop.value.kind === ValueKind.Fixed) return prop.value.data;
		if (prop.value.kind === ValueKind.Choice) return prop.value.data.options?.[0] ?? fallback;
		return fallback;
	}

	function readStringProp(prop: any, fallback: string): string {
		if (!prop || !prop.value) return fallback;
		if (prop.value.kind === ValueKind.Fixed) return prop.value.data ?? fallback;
		if (prop.value.kind === ValueKind.Choice) return prop.value.data.options?.[0] ?? fallback;
		return fallback;
	}

	function fmt(value: number): number {
		return Number(value.toFixed(3));
	}

	function transformPoint(point: Point, orientation: Orientation): Point {
		let x = point.x;
		let y = point.y;

		// Keep designer preview aligned with final font rendering orientation.
		// In the font pipeline there is a vertical axis inversion, so N/S need to be swapped here.
		if (orientation === Orientation.SE || orientation === Orientation.SW) {
			y = 100 - y;
		}
		if (orientation === Orientation.NW || orientation === Orientation.SW) {
			x = 100 - x;
		}

		return { x: fmt(x), y: fmt(y) };
	}

	function quarterPath(squaring: number, negative: boolean, orientation: Orientation): string {
		const s = clamp(squaring, 0, 1);

		const a = transformPoint({ x: 0, y: 0 }, orientation);
		const ah = transformPoint({ x: 100 * s, y: 0 }, orientation);
		const bh = transformPoint({ x: 100, y: 100 - 100 * s }, orientation);
		const b = transformPoint({ x: 100, y: 100 }, orientation);
		const c = transformPoint(negative ? { x: 100, y: 0 } : { x: 0, y: 100 }, orientation);

		return `M ${a.x} ${a.y} C ${ah.x} ${ah.y} ${bh.x} ${bh.y} ${b.x} ${b.y} L ${c.x} ${c.y} Z`;
	}

	function trianglePath(orientation: Orientation): string {
		const a = transformPoint({ x: 0, y: 0 }, orientation);
		const b = transformPoint({ x: 100, y: 100 }, orientation);
		const c = transformPoint({ x: 0, y: 100 }, orientation);
		return `M ${a.x} ${a.y} L ${b.x} ${b.y} L ${c.x} ${c.y} Z`;
	}

	$: kind = rule?.shape.kind ?? ShapeKind.Void;
	$: props = rule?.shape.props as any;
	$: scaleX = readNumberProp(props?.scale_x, 1);
	$: scaleY = readNumberProp(props?.scale_y, 1);
	$: rotation = readNumberProp(props?.rotation, 0);
	$: squaring = readNumberProp(props?.squaring, 0.56);
	$: negative = readBooleanProp(props?.negative, false);
	$: orientation = readOrientationProp(props?.orientation, Orientation.NE);
	$: svgSource = readStringProp(props?.path, '').trim();
	$: hasSvgSource = svgSource.length > 0;
	$: qPath = quarterPath(squaring, negative, orientation);
	$: tPath = trianglePath(orientation);
	$: transform = `translate(50 50) rotate(${rotation}) scale(${scaleX} ${scaleY}) translate(-50 -50)`;
</script>

<svg viewBox="0 0 100 100" class={className} aria-hidden="true">
	<g transform={transform} fill="currentColor">
		{#if kind === ShapeKind.Rectangle}
			<rect x="0" y="0" width="100" height="100" />
		{:else if kind === ShapeKind.Ellipse}
			{#if negative}
				<path
					fill-rule="evenodd"
					d="M 0 0 H 100 V 100 H 0 Z M 50 0 A 50 50 0 1 1 50 100 A 50 50 0 1 1 50 0 Z"
				/>
			{:else}
				<ellipse cx="50" cy="50" rx="50" ry="50" />
			{/if}
		{:else if kind === ShapeKind.Quarter}
			{#if negative}
				<path fill-rule="evenodd" d={`M 0 0 H 100 V 100 H 0 Z ${qPath}`} />
			{:else}
				<path d={qPath} />
			{/if}
		{:else if kind === ShapeKind.Triangle}
			<path d={tPath} />
		{:else if kind === ShapeKind.SVG}
			{#if hasSvgSource}
				<rect x="10" y="10" width="80" height="80" fill="none" stroke="currentColor" stroke-width="10" />
				<path d="M 20 70 L 40 40 L 58 58 L 80 30" fill="none" stroke="currentColor" stroke-width="10" />
			{:else}
				<rect x="0" y="0" width="100" height="100" />
			{/if}
		{/if}
	</g>
</svg>
