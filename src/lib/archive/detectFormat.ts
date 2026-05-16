export type ArchiveFormat = 'chirone' | 'legacy' | 'unknown';

export function detectArchiveFormat(file: File): ArchiveFormat {
	if (file.name.endsWith('.chirone.tar.gz')) return 'chirone';
	if (file.name.endsWith('.json')) return 'legacy';
	return 'unknown';
}
