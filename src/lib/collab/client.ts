import { get, writable } from 'svelte/store';
import { glyphs, metrics, selectedGlyph, syntaxes } from '$lib/stores';
import type { FontMetrics } from '$lib/GTL/metrics';
import { normalizeFontMetrics } from '$lib/GTL/metrics';
import type { GlyphInput, Syntax } from '$lib/types';

type CollabState = 'disabled' | 'connecting' | 'connected' | 'offline' | 'error';

type FontProjectSnapshot = {
	glyphs: Array<GlyphInput>;
	syntaxes: Array<Syntax>;
	metrics: FontMetrics;
};

type ProjectDocument = FontProjectSnapshot & {
	project: string;
	version: number;
	updatedAt: string;
};

type ProjectEvent = ProjectDocument & {
	type: string;
	clientId?: string;
};

type CollabStatus = {
	enabled: boolean;
	state: CollabState;
	project: string;
	version: number;
	message: string;
};

const collabServer = (import.meta.env.VITE_COLLAB_SERVER as string | undefined)?.trim() ?? '';
const collabProject = sanitizeProjectID(
	(import.meta.env.VITE_COLLAB_PROJECT as string | undefined)?.trim() || 'default'
);
const collabEnabled = collabServer.length > 0;

const initialStatus: CollabStatus = {
	enabled: collabEnabled,
	state: collabEnabled ? 'connecting' : 'disabled',
	project: collabProject,
	version: 0,
	message: collabEnabled
		? 'Connecting to collaboration server...'
		: 'Collaboration disabled (set VITE_COLLAB_SERVER)'
};

export const collabStatus = writable<CollabStatus>(initialStatus);

let singletonStop: (() => void) | null = null;

function sanitizeProjectID(raw: string): string {
	return /^[a-zA-Z0-9_-]+$/.test(raw) ? raw : 'default';
}

function isObjectRecord(input: unknown): input is Record<string, unknown> {
	return typeof input === 'object' && input !== null;
}

function coerceMetrics(input: unknown): FontMetrics | null {
	if (!isObjectRecord(input)) return null;

	try {
		return normalizeFontMetrics(input as any);
	} catch {
		return null;
	}
}

function coerceSnapshot(input: unknown): FontProjectSnapshot | null {
	if (!isObjectRecord(input)) return null;

	const glyphList = input.glyphs;
	const syntaxList = input.syntaxes;
	const metricMap = coerceMetrics(input.metrics);

	if (!Array.isArray(glyphList)) return null;
	if (!Array.isArray(syntaxList)) return null;
	if (!metricMap) return null;

	return {
		glyphs: glyphList as Array<GlyphInput>,
		syntaxes: syntaxList as Array<Syntax>,
		metrics: metricMap
	};
}

function coerceProjectDocument(input: unknown): ProjectDocument | null {
	if (!isObjectRecord(input)) return null;

	const snapshot = coerceSnapshot(input);
	if (!snapshot) return null;

	const version = typeof input.version === 'number' ? input.version : 0;
	const updatedAt = typeof input.updatedAt === 'string' ? input.updatedAt : '';
	const project = typeof input.project === 'string' ? sanitizeProjectID(input.project) : collabProject;

	return {
		...snapshot,
		project,
		version,
		updatedAt
	};
}

function coerceProjectEvent(input: unknown): ProjectEvent | null {
	if (!isObjectRecord(input)) return null;
	const document = coerceProjectDocument(input);
	if (!document) return null;

	const type = typeof input.type === 'string' ? input.type : 'snapshot';
	const clientId = typeof input.clientId === 'string' ? input.clientId : undefined;

	return {
		type,
		clientId,
		...document
	};
}

function buildClientID(): string {
	if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
		return crypto.randomUUID();
	}
	return `client-${Math.random().toString(36).slice(2)}`;
}

export function initCollabSync(): () => void {
	if (singletonStop) return singletonStop;
	if (!collabEnabled) {
		collabStatus.set(initialStatus);
		singletonStop = () => {
			singletonStop = null;
		};
		return singletonStop;
	}

	const stop = startCollabRuntime(collabServer.replace(/\/+$/g, ''), collabProject);
	singletonStop = () => {
		stop();
		singletonStop = null;
	};
	return singletonStop;
}

function startCollabRuntime(serverBase: string, projectID: string): () => void {
	const clientID = buildClientID();
	const projectURL = `${serverBase}/api/project?project=${encodeURIComponent(projectID)}`;
	const eventsURL = `${serverBase}/api/events?project=${encodeURIComponent(projectID)}`;

	let stopped = false;
	let lastVersion = 0;
	let isApplyingRemote = false;
	let localSyncReady = false;
	let reconnectAttempts = 0;
	let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	let pushTimer: ReturnType<typeof setTimeout> | undefined;
	let inFlightPush = false;
	let pendingPush = false;
	let eventSource: EventSource | null = null;

	const unsubs: Array<() => void> = [];

	const setStatus = (state: CollabState, message: string) => {
		collabStatus.set({
			enabled: true,
			state,
			project: projectID,
			version: lastVersion,
			message
		});
	};

	const readLocalSnapshot = (): FontProjectSnapshot => ({
		glyphs: get(glyphs),
		syntaxes: get(syntaxes),
		metrics: get(metrics)
	});

	const applyRemoteSnapshot = (snapshot: FontProjectSnapshot, nextVersion: number) => {
		isApplyingRemote = true;
		glyphs.set(snapshot.glyphs);
		syntaxes.set(snapshot.syntaxes);
		metrics.set(snapshot.metrics);

		const currentSelectedGlyph = get(selectedGlyph);
		if (currentSelectedGlyph && !snapshot.glyphs.some((glyph) => glyph.id === currentSelectedGlyph)) {
			selectedGlyph.set(snapshot.glyphs[0]?.id ?? '');
		}
		isApplyingRemote = false;
		lastVersion = Math.max(lastVersion, nextVersion);
	};

	const scheduleReconnect = () => {
		if (stopped || reconnectTimer) return;
		const delay = Math.min(10000, 500 * 2 ** reconnectAttempts);
		reconnectAttempts += 1;
		reconnectTimer = setTimeout(() => {
			reconnectTimer = undefined;
			connectSSE();
		}, delay);
	};

	const schedulePush = (delay = 250) => {
		if (stopped || !localSyncReady || isApplyingRemote) return;
		if (pushTimer) clearTimeout(pushTimer);
		pushTimer = setTimeout(() => {
			pushTimer = undefined;
			void pushSnapshotNow();
		}, delay);
	};

	const maybeSchedulePush = () => {
		if (!localSyncReady || isApplyingRemote) return;
		schedulePush(250);
	};

	const pushSnapshotNow = async () => {
		if (stopped) return;
		if (inFlightPush) {
			pendingPush = true;
			return;
		}
		inFlightPush = true;

		try {
			const response = await fetch(projectURL, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					clientId: clientID,
					...readLocalSnapshot()
				})
			});

			if (!response.ok) {
				throw new Error(`sync push failed: ${response.status}`);
			}

			const payload = (await response.json()) as unknown;
			const document = coerceProjectDocument(payload);
			if (!document) {
				throw new Error('sync push failed: invalid response payload');
			}
			lastVersion = Math.max(lastVersion, document.version);
			setStatus('connected', `Synced (v${lastVersion})`);
		} catch (error) {
			setStatus('offline', error instanceof Error ? error.message : 'sync push failed');
			scheduleReconnect();
		} finally {
			inFlightPush = false;
			if (pendingPush) {
				pendingPush = false;
				schedulePush(100);
			}
		}
	};

	const connectSSE = () => {
		if (stopped) return;
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}

		setStatus('connecting', `Connecting stream for "${projectID}"...`);

		const es = new EventSource(eventsURL);
		eventSource = es;

		es.onopen = () => {
			reconnectAttempts = 0;
			setStatus('connected', `Realtime sync active (v${lastVersion})`);
		};

		es.addEventListener('snapshot', (event) => {
			if (stopped) return;
			let payload: unknown;
			try {
				payload = JSON.parse((event as MessageEvent).data);
			} catch {
				return;
			}
			const update = coerceProjectEvent(payload);
			if (!update) return;
			if (update.clientId && update.clientId === clientID) return;
			if (update.version <= lastVersion) return;
			applyRemoteSnapshot(update, update.version);
			setStatus('connected', `Received update (v${lastVersion})`);
		});

		es.onerror = () => {
			if (stopped) return;
			setStatus('offline', 'Realtime stream disconnected, retrying...');
			es.close();
			if (eventSource === es) {
				eventSource = null;
			}
			scheduleReconnect();
		};
	};

	const bootstrap = async () => {
		let loadedRemote = false;
		setStatus('connecting', `Loading project "${projectID}"...`);

		try {
			const response = await fetch(projectURL);
			if (response.status === 404) {
				loadedRemote = false;
			} else if (!response.ok) {
				throw new Error(`load failed: ${response.status}`);
			} else {
				const payload = (await response.json()) as unknown;
				const document = coerceProjectDocument(payload);
				if (!document) {
					throw new Error('load failed: invalid response payload');
				}
				applyRemoteSnapshot(document, document.version);
				loadedRemote = true;
				setStatus('connected', `Loaded snapshot (v${lastVersion})`);
			}
		} catch (error) {
			setStatus('offline', error instanceof Error ? error.message : 'load failed');
		}

		unsubs.push(glyphs.subscribe(maybeSchedulePush));
		unsubs.push(syntaxes.subscribe(maybeSchedulePush));
		unsubs.push(metrics.subscribe(maybeSchedulePush));
		localSyncReady = true;

		if (!loadedRemote) {
			schedulePush(0);
		}

		connectSSE();
	};

	void bootstrap();

	return () => {
		stopped = true;
		localSyncReady = false;
		for (const unsub of unsubs) {
			unsub();
		}
		if (pushTimer) clearTimeout(pushTimer);
		if (reconnectTimer) clearTimeout(reconnectTimer);
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		setStatus('disabled', 'Collaboration stopped');
	};
}
