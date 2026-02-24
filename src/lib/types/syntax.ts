import type { Shape } from './shapes';

/**
 * Rule -> Syntax
 */

export type Rule = {
	symbol: string;
	shape: Shape;
	unused?: boolean;
};

export type Syntax = {
	id: string;
	name: string;
	rules: Array<Rule>;
	grid: {
		rows: number;
		columns: number;
	};
};
