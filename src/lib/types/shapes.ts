import type { BaseProps, BooleanProp, CurveProps, OrientationProp, StringProp } from './props';

/**
 * Shapes
 */

export enum ShapeKind {
	Void = 'void',
	Rectangle = 'rectangle',
	Circle = 'circle',
	Ellipse = 'ellipse',
	Quarter = 'quarter',
	Triangle = 'triangle',
	SVG = 'svg'
}

export interface ShapeTemplate<K, P> {
	kind: K;
	props: P;
}

// Void

export type VoidProps = Record<string, unknown>;
export type VoidShape = ShapeTemplate<ShapeKind.Void, VoidProps>;

// Rectangle

export type RectangleProps = BaseProps;
export type RectangleShape = ShapeTemplate<ShapeKind.Rectangle, RectangleProps>;

// Circle

export type CircleProps = BaseProps;
export type CircleShape = ShapeTemplate<ShapeKind.Circle, CircleProps>;

// Ellipse

export type EllipseProps = BaseProps & CurveProps;
export type EllipseShape = ShapeTemplate<ShapeKind.Ellipse, EllipseProps>;

// Quarter

export type QuarterProps = BaseProps &
	CurveProps & {
		orientation: OrientationProp;
	};

export type QuarterShape = ShapeTemplate<ShapeKind.Quarter, QuarterProps>;

// Triangle

export type TriangleProps = BaseProps & {
	orientation: OrientationProp;
};

export type TriangleShape = ShapeTemplate<ShapeKind.Triangle, TriangleProps>;

// SVG

export type SVGProps = BaseProps & {
	path: StringProp;
	negative: BooleanProp;
};
export type SVGShape = ShapeTemplate<ShapeKind.SVG, SVGProps>;

/**
 * Utility union types
 */

export type Props =
	| VoidProps
	| RectangleProps
	| CircleProps
	| EllipseProps
	| QuarterProps
	| TriangleProps
	| SVGProps;
export type Shape =
	| VoidShape
	| RectangleShape
	| CircleShape
	| EllipseShape
	| QuarterShape
	| TriangleShape
	| SVGShape;
