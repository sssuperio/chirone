const BLOCK_SIZE = 512;

export interface TarEntry {
	name: string;
	content: Uint8Array;
}

function padBlock(buf: Uint8Array): Uint8Array {
	const remainder = buf.length % BLOCK_SIZE;
	if (remainder === 0) return buf;
	const padded = new Uint8Array(buf.length + (BLOCK_SIZE - remainder));
	padded.set(buf);
	return padded;
}

function octal(num: number, length: number): string {
	return num.toString(8).padStart(length - 1, '0') + '\x00';
}

function checksum(header: Uint8Array): number {
	let sum = 0;
	for (let i = 0; i < BLOCK_SIZE; i++) {
		if (i >= 148 && i < 156) {
			sum += 32; // space
		} else {
			sum += header[i] | 0;
		}
	}
	return sum;
}

function createHeader(name: string, size: number): Uint8Array {
	const header = new Uint8Array(BLOCK_SIZE);
	const encoder = new TextEncoder();

	if (name.length > 99) {
		name = name.slice(0, 99);
	}

	const write = (offset: number, value: string) => {
		const bytes = encoder.encode(value);
		header.set(bytes, offset);
	};

	write(0, name.padEnd(100, '\x00'));
	write(100, octal(0o644, 8));
	write(108, octal(0, 8));
	write(116, octal(0, 8));
	write(124, octal(size, 12));
	write(136, octal(Math.floor(Date.now() / 1000), 12));
	write(148, '        ');
	header[156] = 48; // '0' regular file
	write(257, 'ustar\x0000');
	write(265, '00');
	write(345, '');

	// Compute and write checksum
	const sum = checksum(header);
	write(148, octal(sum, 7) + ' ');

	return header;
}

export function buildTar(entries: TarEntry[]): Uint8Array {
	const blocks: Uint8Array[] = [];

	for (const entry of entries) {
		const header = createHeader(entry.name, entry.content.length);
		blocks.push(header);

		if (entry.content.length > 0) {
			blocks.push(padBlock(entry.content));
		}
	}

	// Two zero blocks to mark end of archive
	const endBlock = new Uint8Array(BLOCK_SIZE * 2);
	blocks.push(endBlock);

	const totalSize = blocks.reduce((acc, b) => acc + b.length, 0);
	const result = new Uint8Array(totalSize);
	let offset = 0;
	for (const block of blocks) {
		result.set(block, offset);
		offset += block.length;
	}
	return result;
}

export function parseTar(raw: Uint8Array): TarEntry[] {
	const entries: TarEntry[] = [];
	let offset = 0;

	while (offset + BLOCK_SIZE <= raw.length) {
		// Check for zero block (end of archive)
		let allZero = true;
		for (let i = offset; i < offset + BLOCK_SIZE && i < raw.length; i++) {
			if (raw[i] !== 0) {
				allZero = false;
				break;
			}
		}
		if (allZero) break;

		// Read filename
		const decoder = new TextDecoder();
		let nameEnd = offset;
		while (raw[nameEnd] !== 0 && nameEnd < offset + 100) nameEnd++;
		const name = decoder.decode(raw.slice(offset, nameEnd));

		// Read size (12 bytes octal string at offset+124)
		const sizeStr = decoder.decode(raw.slice(offset + 124, offset + 136));
		const size = parseInt(sizeStr.trim().split('\x00')[0], 8) || 0;

		if (size > 0 && name) {
			const contentStart = offset + BLOCK_SIZE;
			const contentEnd = contentStart + size;
			if (contentEnd <= raw.length) {
				entries.push({
					name,
					content: raw.slice(contentStart, contentEnd)
				});
			}
		}

		// Advance to next entry (content padded to block boundary)
		const paddedSize = size > 0 ? Math.ceil(size / BLOCK_SIZE) * BLOCK_SIZE : 0;
		offset += BLOCK_SIZE + paddedSize;
	}

	return entries;
}
