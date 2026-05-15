import type { FontMetrics } from '$lib/GTL/metrics';
import type { FontMetadata } from '$lib/GTL/metadata';

export interface MetricsPreset extends FontMetrics {
	id: string;
	name: string;
}

export interface MetadataPreset extends FontMetadata {
	id: string;
	name: string;
}

export interface FontDefinition {
	id: string;
	name: string;
	syntaxId: string;
	metricsId: string;
	metadataId: string;
	glyphIds?: string[];
	outputName: string;
	enabled: boolean;
}

export interface ProjectInfo {
	id: string;
	name: string;
	createdAt: string;
	updatedAt: string;
}
