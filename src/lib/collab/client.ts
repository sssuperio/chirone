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

type ProjectResponse = ProjectDocument & {
	glyphVersions?: Record<string, number>;
	syntaxVersions?: Record<string, number>;
	metricsVersion?: number;
};

type EntitySyncResponse = {
	project: string;
	entity: 'glyph' | 'syntax' | 'metrics';
	entityId?: string;
	version: number;
	projectVersion?: number;
	deleted?: boolean;
	updatedAt: string;
	payload?: unknown;
};

type EntityEvent = {
	type: string;
	clientId?: string;
	entity: 'glyph' | 'syntax' | 'metrics';
	entityId?: string;
	entityVersion: number;
	entityDeleted: boolean;
	payload?: unknown;
	version: number;
};

type CollabStatus = {
	enabled: boolean;
	state: CollabState;
	project: string;
	version: number;
	message: string;
};

const collabServer = (import.meta.env.VITE_COLLAB_SERVER as string | undefined)?.trim() ?? '';
const collabProjectDefault = sanitizeProjectID(
	(import.meta.env.VITE_COLLAB_PROJECT as string | undefined)?.trim() || 'default'
);
const collabEnabled = collabServer.length > 0;
const collabServerBase = collabServer.replace(/\/+$/g, '');
const collabProjectStorageKey = 'chirone-collab-project';
let activeCollabProject = loadCollabProject(collabProjectDefault);
let runtimeStop: (() => void) | null = null;

const initialStatus = buildInitialStatus(activeCollabProject);

export const collabStatus = writable<CollabStatus>(initialStatus);

let singletonStop: (() => void) | null = null;

function sanitizeProjectID(raw: string): string {
	return /^[a-zA-Z0-9_-]+$/.test(raw) ? raw : 'default';
}

function isObjectRecord(input: unknown): input is Record<string, unknown> {
	return typeof input === 'object' && input !== null;
}

function loadCollabProject(defaultProject: string): string {
	if (typeof window === 'undefined') return defaultProject;
	try {
		const raw = window.localStorage.getItem(collabProjectStorageKey);
		if (!raw) return defaultProject;
		return sanitizeProjectID(raw.trim() || defaultProject);
	} catch {
		return defaultProject;
	}
}

function persistCollabProject(project: string) {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(collabProjectStorageKey, project);
	} catch {
		// Ignore storage failures; runtime project still changes in-memory.
	}
}

function buildInitialStatus(project: string): CollabStatus {
	return {
		enabled: collabEnabled,
		state: collabEnabled ? 'connecting' : 'disabled',
		project,
		version: 0,
		message: collabEnabled
			? 'Connecting to collaboration server...'
			: 'Collaboration disabled (set VITE_COLLAB_SERVER)'
	};
}

function stableStringify(input: unknown): string {
	try {
		return JSON.stringify(input) ?? '';
	} catch {
		return '';
	}
}

function coerceMetrics(input: unknown): FontMetrics | null {
	if (!isObjectRecord(input)) return null;

	try {
		return normalizeFontMetrics(input as any);
	} catch {
		return null;
	}
}

function coerceGlyph(input: unknown): GlyphInput | null {
	if (!isObjectRecord(input)) return null;
	if (typeof input.id !== 'string' || !input.id.trim()) return null;
	if (typeof input.name !== 'string') return null;
	if (typeof input.structure !== 'string') return null;
	return input as unknown as GlyphInput;
}

function coerceSyntax(input: unknown): Syntax | null {
	if (!isObjectRecord(input)) return null;
	if (typeof input.id !== 'string' || !input.id.trim()) return null;
	return input as unknown as Syntax;
}

function coerceSnapshot(input: unknown): FontProjectSnapshot | null {
	if (!isObjectRecord(input)) return null;

	const glyphList = input.glyphs;
	const syntaxList = input.syntaxes;
	const metricMap = coerceMetrics(input.metrics);

	if (!Array.isArray(glyphList)) return null;
	if (!Array.isArray(syntaxList)) return null;
	if (!metricMap) return null;

	const parsedGlyphs: Array<GlyphInput> = [];
	for (const item of glyphList) {
		const glyph = coerceGlyph(item);
		if (!glyph) return null;
		parsedGlyphs.push(glyph);
	}

	const parsedSyntaxes: Array<Syntax> = [];
	for (const item of syntaxList) {
		const syntax = coerceSyntax(item);
		if (!syntax) return null;
		parsedSyntaxes.push(syntax);
	}

	return {
		glyphs: parsedGlyphs,
		syntaxes: parsedSyntaxes,
		metrics: metricMap
	};
}

function coerceVersionMap(input: unknown): Record<string, number> {
	if (!isObjectRecord(input)) return {};
	const out: Record<string, number> = {};
	for (const [key, value] of Object.entries(input)) {
		if (typeof value === 'number' && Number.isFinite(value) && value >= 0) {
			out[key] = Math.trunc(value);
		}
	}
	return out;
}

function coerceProjectResponse(input: unknown): ProjectResponse | null {
	if (!isObjectRecord(input)) return null;

	const snapshot = coerceSnapshot(input);
	if (!snapshot) return null;

	const version = typeof input.version === 'number' ? input.version : 0;
	const updatedAt = typeof input.updatedAt === 'string' ? input.updatedAt : '';
	const project =
		typeof input.project === 'string' ? sanitizeProjectID(input.project) : activeCollabProject;

	return {
		...snapshot,
		project,
		version,
		updatedAt,
		glyphVersions: coerceVersionMap(input.glyphVersions),
		syntaxVersions: coerceVersionMap(input.syntaxVersions),
		metricsVersion:
			typeof input.metricsVersion === 'number' && Number.isFinite(input.metricsVersion)
				? Math.max(0, Math.trunc(input.metricsVersion))
				: 0
	};
}

function coerceEntitySyncResponse(input: unknown): EntitySyncResponse | null {
	if (!isObjectRecord(input)) return null;
	const entity = input.entity;
	if (entity !== 'glyph' && entity !== 'syntax' && entity !== 'metrics') return null;
	if (typeof input.project !== 'string') return null;
	if (typeof input.version !== 'number' || !Number.isFinite(input.version)) return null;
	if (typeof input.updatedAt !== 'string') return null;

	const entityId = typeof input.entityId === 'string' ? input.entityId : undefined;
	const deleted = Boolean(input.deleted);
	const projectVersion =
		typeof input.projectVersion === 'number' && Number.isFinite(input.projectVersion)
			? Math.max(0, Math.trunc(input.projectVersion))
			: undefined;

	return {
		project: sanitizeProjectID(input.project),
		entity,
		entityId,
		version: Math.max(0, Math.trunc(input.version)),
		projectVersion,
		deleted,
		updatedAt: input.updatedAt,
		payload: input.payload
	};
}

function coerceEntityEvent(input: unknown): EntityEvent | null {
	if (!isObjectRecord(input)) return null;
	const type = typeof input.type === 'string' ? input.type : '';
	const entity = input.entity;
	if (entity !== 'glyph' && entity !== 'syntax' && entity !== 'metrics') return null;
	if (typeof input.entityVersion !== 'number' || !Number.isFinite(input.entityVersion)) return null;

	const entityId = typeof input.entityId === 'string' ? input.entityId : undefined;
	const clientId = typeof input.clientId === 'string' ? input.clientId : undefined;
	const version = typeof input.version === 'number' && Number.isFinite(input.version) ? input.version : 0;

	return {
		type,
		clientId,
		entity,
		entityId,
		entityVersion: Math.max(0, Math.trunc(input.entityVersion)),
		entityDeleted: Boolean(input.entityDeleted),
		payload: input.payload,
		version: Math.max(0, Math.trunc(version))
	};
}

function buildClientID(): string {
	if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
		return crypto.randomUUID();
	}
	return `client-${Math.random().toString(36).slice(2)}`;
}

function cloneGlyph(input: GlyphInput): GlyphInput {
	return JSON.parse(JSON.stringify(input)) as GlyphInput;
}

function cloneSyntax(input: Syntax): Syntax {
	return JSON.parse(JSON.stringify(input)) as Syntax;
}

function cloneMetrics(input: FontMetrics): FontMetrics {
	return normalizeFontMetrics(JSON.parse(JSON.stringify(input)) as any);
}

export function initCollabSync(): () => void {
	if (!singletonStop) {
		singletonStop = () => {
			stopRuntime();
			singletonStop = null;
		};
	}

	if (!runtimeStop) {
		if (!collabEnabled) {
			collabStatus.set(buildInitialStatus(activeCollabProject));
		} else {
			runtimeStop = startCollabRuntime(collabServerBase, activeCollabProject);
		}
	}

	return singletonStop;
}

export function setCollabProject(nextProjectRaw: string): string {
	const nextProject = sanitizeProjectID(nextProjectRaw.trim() || 'default');
	if (nextProject === activeCollabProject) return nextProject;

	activeCollabProject = nextProject;
	persistCollabProject(activeCollabProject);

	if (singletonStop) {
		stopRuntime();
		if (collabEnabled) {
			runtimeStop = startCollabRuntime(collabServerBase, activeCollabProject);
		} else {
			collabStatus.set(buildInitialStatus(activeCollabProject));
		}
	} else {
		collabStatus.update((status) => ({
			...status,
			project: activeCollabProject
		}));
	}

	return nextProject;
}

function stopRuntime() {
	if (!runtimeStop) return;
	const stop = runtimeStop;
	runtimeStop = null;
	stop();
}

function startCollabRuntime(serverBase: string, projectID: string): () => void {
	const clientID = buildClientID();
	const projectURL = `${serverBase}/api/project?project=${encodeURIComponent(projectID)}`;
	const glyphURL = `${serverBase}/api/glyph?project=${encodeURIComponent(projectID)}`;
	const syntaxURL = `${serverBase}/api/syntax?project=${encodeURIComponent(projectID)}`;
	const metricsURL = `${serverBase}/api/metrics?project=${encodeURIComponent(projectID)}`;
	const eventsURL = `${serverBase}/api/events?project=${encodeURIComponent(projectID)}`;

	let stopped = false;
	let lastVersion = 0;
	let isApplyingRemote = false;
	let localSyncReady = false;
	let reconnectAttempts = 0;
	let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	let pushTimer: ReturnType<typeof setTimeout> | undefined;
	let inFlightReload: Promise<boolean> | null = null;
	let inFlightPush = false;
	let pendingPush = false;
	let eventSource: EventSource | null = null;

	let glyphVersions = new Map<string, number>();
	let syntaxVersions = new Map<string, number>();
	let metricsVersion = 0;

	let knownGlyphHashes = new Map<string, string>();
	let knownSyntaxHashes = new Map<string, string>();
	let knownMetricsHash = '';

	const pendingGlyphUpserts = new Map<string, GlyphInput>();
	const pendingGlyphDeletes = new Set<string>();
	const pendingSyntaxUpserts = new Map<string, Syntax>();
	const pendingSyntaxDeletes = new Set<string>();
	let pendingMetrics: FontMetrics | null = null;

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

	const ensureSelectedGlyph = (nextGlyphs: Array<GlyphInput>) => {
		const currentSelectedGlyph = get(selectedGlyph);
		if (currentSelectedGlyph && !nextGlyphs.some((glyph) => glyph.id === currentSelectedGlyph)) {
			selectedGlyph.set(nextGlyphs[0]?.id ?? '');
		}
	};

	const clearPendingOps = () => {
		pendingGlyphUpserts.clear();
		pendingGlyphDeletes.clear();
		pendingSyntaxUpserts.clear();
		pendingSyntaxDeletes.clear();
		pendingMetrics = null;
	};

	const refreshLocalHashes = () => {
		knownGlyphHashes = new Map(get(glyphs).map((glyph) => [glyph.id, stableStringify(glyph)]));
		knownSyntaxHashes = new Map(get(syntaxes).map((syntax) => [syntax.id, stableStringify(syntax)]));
		knownMetricsHash = stableStringify(get(metrics));
	};

	const applyVersionMapsFromSnapshot = (snapshot: FontProjectSnapshot, response?: ProjectResponse) => {
		if (response) {
			glyphVersions = new Map(Object.entries(response.glyphVersions ?? {}));
			syntaxVersions = new Map(Object.entries(response.syntaxVersions ?? {}));
			metricsVersion = Math.max(0, response.metricsVersion ?? 0);
			return;
		}

		const nextGlyphVersions = new Map<string, number>();
		for (const glyph of snapshot.glyphs) {
			nextGlyphVersions.set(glyph.id, Math.max(1, glyphVersions.get(glyph.id) ?? 1));
		}
		glyphVersions = nextGlyphVersions;

		const nextSyntaxVersions = new Map<string, number>();
		for (const syntax of snapshot.syntaxes) {
			nextSyntaxVersions.set(syntax.id, Math.max(1, syntaxVersions.get(syntax.id) ?? 1));
		}
		syntaxVersions = nextSyntaxVersions;
		metricsVersion = Math.max(1, metricsVersion || 1);
	};

	const applyRemoteSnapshot = (
		snapshot: FontProjectSnapshot,
		nextVersion: number,
		response?: ProjectResponse
	) => {
		isApplyingRemote = true;
		glyphs.set(snapshot.glyphs);
		syntaxes.set(snapshot.syntaxes);
		metrics.set(snapshot.metrics);
		ensureSelectedGlyph(snapshot.glyphs);
		isApplyingRemote = false;

		lastVersion = Math.max(lastVersion, nextVersion);
		applyVersionMapsFromSnapshot(snapshot, response);
		refreshLocalHashes();
		clearPendingOps();
	};

	const applyRemoteGlyphUpsert = (glyph: GlyphInput, entityVersion: number, globalVersion: number) => {
		const current = get(glyphs);
		const index = current.findIndex((item) => item.id === glyph.id);
		const next = [...current];
		if (index >= 0) {
			next[index] = glyph;
		} else {
			next.push(glyph);
		}

		isApplyingRemote = true;
		glyphs.set(next);
		ensureSelectedGlyph(next);
		isApplyingRemote = false;

		glyphVersions.set(glyph.id, entityVersion);
		pendingGlyphUpserts.delete(glyph.id);
		pendingGlyphDeletes.delete(glyph.id);
		lastVersion = Math.max(lastVersion, globalVersion);
		refreshLocalHashes();
	};

	const applyRemoteGlyphDelete = (glyphID: string, _entityVersion: number, globalVersion: number) => {
		const next = get(glyphs).filter((item) => item.id !== glyphID);
		isApplyingRemote = true;
		glyphs.set(next);
		ensureSelectedGlyph(next);
		isApplyingRemote = false;

		glyphVersions.delete(glyphID);
		pendingGlyphUpserts.delete(glyphID);
		pendingGlyphDeletes.delete(glyphID);
		lastVersion = Math.max(lastVersion, globalVersion);
		refreshLocalHashes();
	};

	const applyRemoteSyntaxUpsert = (syntax: Syntax, entityVersion: number, globalVersion: number) => {
		const current = get(syntaxes);
		const index = current.findIndex((item) => item.id === syntax.id);
		const next = [...current];
		if (index >= 0) {
			next[index] = syntax;
		} else {
			next.push(syntax);
		}

		isApplyingRemote = true;
		syntaxes.set(next);
		isApplyingRemote = false;

		syntaxVersions.set(syntax.id, entityVersion);
		pendingSyntaxUpserts.delete(syntax.id);
		pendingSyntaxDeletes.delete(syntax.id);
		lastVersion = Math.max(lastVersion, globalVersion);
		refreshLocalHashes();
	};

	const applyRemoteSyntaxDelete = (syntaxID: string, _entityVersion: number, globalVersion: number) => {
		const next = get(syntaxes).filter((item) => item.id !== syntaxID);
		isApplyingRemote = true;
		syntaxes.set(next);
		isApplyingRemote = false;

		syntaxVersions.delete(syntaxID);
		pendingSyntaxUpserts.delete(syntaxID);
		pendingSyntaxDeletes.delete(syntaxID);
		lastVersion = Math.max(lastVersion, globalVersion);
		refreshLocalHashes();
	};

	const applyRemoteMetricsUpdate = (nextMetrics: FontMetrics, entityVersion: number, globalVersion: number) => {
		isApplyingRemote = true;
		metrics.set(nextMetrics);
		isApplyingRemote = false;

		metricsVersion = entityVersion;
		pendingMetrics = null;
		lastVersion = Math.max(lastVersion, globalVersion);
		refreshLocalHashes();
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

	const schedulePush = (delay = 200) => {
		if (stopped || !localSyncReady || isApplyingRemote) return;
		if (pushTimer) clearTimeout(pushTimer);
		pushTimer = setTimeout(() => {
			pushTimer = undefined;
			void flushPendingOps();
		}, delay);
	};

	const syncLocalGlyphQueue = (currentGlyphs: Array<GlyphInput>) => {
		const nextHashes = new Map<string, string>();
		for (const glyph of currentGlyphs) {
			const hash = stableStringify(glyph);
			nextHashes.set(glyph.id, hash);
			if (knownGlyphHashes.get(glyph.id) !== hash && !isApplyingRemote) {
				pendingGlyphUpserts.set(glyph.id, cloneGlyph(glyph));
				pendingGlyphDeletes.delete(glyph.id);
			}
		}
		for (const glyphID of knownGlyphHashes.keys()) {
			if (!nextHashes.has(glyphID) && !isApplyingRemote) {
				pendingGlyphDeletes.add(glyphID);
				pendingGlyphUpserts.delete(glyphID);
			}
		}
		knownGlyphHashes = nextHashes;
		if (!isApplyingRemote) {
			schedulePush(180);
		}
	};

	const syncLocalSyntaxQueue = (currentSyntaxes: Array<Syntax>) => {
		const nextHashes = new Map<string, string>();
		for (const syntax of currentSyntaxes) {
			const hash = stableStringify(syntax);
			nextHashes.set(syntax.id, hash);
			if (knownSyntaxHashes.get(syntax.id) !== hash && !isApplyingRemote) {
				pendingSyntaxUpserts.set(syntax.id, cloneSyntax(syntax));
				pendingSyntaxDeletes.delete(syntax.id);
			}
		}
		for (const syntaxID of knownSyntaxHashes.keys()) {
			if (!nextHashes.has(syntaxID) && !isApplyingRemote) {
				pendingSyntaxDeletes.add(syntaxID);
				pendingSyntaxUpserts.delete(syntaxID);
			}
		}
		knownSyntaxHashes = nextHashes;
		if (!isApplyingRemote) {
			schedulePush(180);
		}
	};

	const syncLocalMetricsQueue = (currentMetrics: FontMetrics) => {
		const hash = stableStringify(currentMetrics);
		if (knownMetricsHash !== hash && !isApplyingRemote) {
			pendingMetrics = cloneMetrics(currentMetrics);
			schedulePush(180);
		}
		knownMetricsHash = hash;
	};

	const queueFullLocalState = () => {
		pendingGlyphUpserts.clear();
		pendingGlyphDeletes.clear();
		for (const glyph of get(glyphs)) {
			pendingGlyphUpserts.set(glyph.id, cloneGlyph(glyph));
		}

		pendingSyntaxUpserts.clear();
		pendingSyntaxDeletes.clear();
		for (const syntax of get(syntaxes)) {
			pendingSyntaxUpserts.set(syntax.id, cloneSyntax(syntax));
		}

		pendingMetrics = cloneMetrics(get(metrics));
		schedulePush(0);
	};

	type PendingOperation =
		| { type: 'glyph_delete'; id: string }
		| { type: 'glyph_upsert'; id: string; glyph: GlyphInput }
		| { type: 'syntax_delete'; id: string }
		| { type: 'syntax_upsert'; id: string; syntax: Syntax }
		| { type: 'metrics_update'; metrics: FontMetrics };

	const takeNextOperation = (): PendingOperation | null => {
		const glyphDelete = pendingGlyphDeletes.values().next().value as string | undefined;
		if (glyphDelete) {
			pendingGlyphDeletes.delete(glyphDelete);
			return { type: 'glyph_delete', id: glyphDelete };
		}

		const glyphUpsert = pendingGlyphUpserts.entries().next().value as [string, GlyphInput] | undefined;
		if (glyphUpsert) {
			pendingGlyphUpserts.delete(glyphUpsert[0]);
			return { type: 'glyph_upsert', id: glyphUpsert[0], glyph: cloneGlyph(glyphUpsert[1]) };
		}

		const syntaxDelete = pendingSyntaxDeletes.values().next().value as string | undefined;
		if (syntaxDelete) {
			pendingSyntaxDeletes.delete(syntaxDelete);
			return { type: 'syntax_delete', id: syntaxDelete };
		}

		const syntaxUpsert = pendingSyntaxUpserts.entries().next().value as [string, Syntax] | undefined;
		if (syntaxUpsert) {
			pendingSyntaxUpserts.delete(syntaxUpsert[0]);
			return { type: 'syntax_upsert', id: syntaxUpsert[0], syntax: cloneSyntax(syntaxUpsert[1]) };
		}

		if (pendingMetrics) {
			const next = pendingMetrics;
			pendingMetrics = null;
			return { type: 'metrics_update', metrics: cloneMetrics(next) };
		}

		return null;
	};

	const restoreOperation = (op: PendingOperation) => {
		switch (op.type) {
			case 'glyph_delete':
				pendingGlyphDeletes.add(op.id);
				break;
			case 'glyph_upsert':
				pendingGlyphUpserts.set(op.id, op.glyph);
				break;
			case 'syntax_delete':
				pendingSyntaxDeletes.add(op.id);
				break;
			case 'syntax_upsert':
				pendingSyntaxUpserts.set(op.id, op.syntax);
				break;
			case 'metrics_update':
				pendingMetrics = op.metrics;
				break;
		}
	};

	const handleEntityConflictResponse = (response: EntitySyncResponse) => {
		const globalVersion = response.projectVersion ?? lastVersion;
		lastVersion = Math.max(lastVersion, globalVersion);

		if (response.entity === 'glyph') {
			const glyphID = response.entityId;
			if (!glyphID) return;
			if (response.deleted) {
				applyRemoteGlyphDelete(glyphID, response.version, globalVersion);
				glyphVersions.delete(glyphID);
			} else {
				const glyph = coerceGlyph(response.payload);
				if (glyph) {
					applyRemoteGlyphUpsert(glyph, response.version, globalVersion);
				}
			}
			return;
		}

		if (response.entity === 'syntax') {
			const syntaxID = response.entityId;
			if (!syntaxID) return;
			if (response.deleted) {
				applyRemoteSyntaxDelete(syntaxID, response.version, globalVersion);
				syntaxVersions.delete(syntaxID);
			} else {
				const syntax = coerceSyntax(response.payload);
				if (syntax) {
					applyRemoteSyntaxUpsert(syntax, response.version, globalVersion);
				}
			}
			return;
		}

		const nextMetrics = coerceMetrics(response.payload);
		if (nextMetrics) {
			applyRemoteMetricsUpdate(nextMetrics, response.version, globalVersion);
		}
	};

	const executeOperation = async (op: PendingOperation): Promise<boolean> => {
		try {
			let response: Response;
			switch (op.type) {
				case 'glyph_upsert': {
					const baseVersion = glyphVersions.get(op.id) ?? 0;
					response = await fetch(glyphURL, {
						method: 'PUT',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							clientId: clientID,
							baseVersion,
							glyph: op.glyph
						})
					});
					break;
				}
				case 'glyph_delete': {
					const baseVersion = glyphVersions.get(op.id) ?? 0;
					response = await fetch(glyphURL, {
						method: 'DELETE',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							clientId: clientID,
							baseVersion,
							id: op.id
						})
					});
					break;
				}
				case 'syntax_upsert': {
					const baseVersion = syntaxVersions.get(op.id) ?? 0;
					response = await fetch(syntaxURL, {
						method: 'PUT',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							clientId: clientID,
							baseVersion,
							syntax: op.syntax
						})
					});
					break;
				}
				case 'syntax_delete': {
					const baseVersion = syntaxVersions.get(op.id) ?? 0;
					response = await fetch(syntaxURL, {
						method: 'DELETE',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							clientId: clientID,
							baseVersion,
							id: op.id
						})
					});
					break;
				}
				case 'metrics_update': {
					response = await fetch(metricsURL, {
						method: 'PUT',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							clientId: clientID,
							baseVersion: metricsVersion,
							metrics: op.metrics
						})
					});
					break;
				}
			}

			if (response.status === 409) {
				const payload = (await response.json().catch(() => null)) as unknown;
				const conflict = coerceEntitySyncResponse(payload);
				if (conflict) {
					handleEntityConflictResponse(conflict);
				}
				setStatus('error', 'Version conflict detected; reloaded conflicting entity');
				return true;
			}

			if (!response.ok) {
				throw new Error(`sync push failed: ${response.status}`);
			}

			const payload = (await response.json()) as unknown;
			const ok = coerceEntitySyncResponse(payload);
			if (!ok) {
				throw new Error('sync push failed: invalid response payload');
			}
			if (typeof ok.projectVersion === 'number') {
				lastVersion = Math.max(lastVersion, ok.projectVersion);
			}

			if (ok.entity === 'glyph' && ok.entityId) {
				if (ok.deleted) {
					glyphVersions.delete(ok.entityId);
				} else {
					glyphVersions.set(ok.entityId, ok.version);
				}
			} else if (ok.entity === 'syntax' && ok.entityId) {
				if (ok.deleted) {
					syntaxVersions.delete(ok.entityId);
				} else {
					syntaxVersions.set(ok.entityId, ok.version);
				}
			} else if (ok.entity === 'metrics') {
				metricsVersion = ok.version;
			}

			setStatus('connected', 'Synced');
			return true;
		} catch (error) {
			setStatus('offline', error instanceof Error ? error.message : 'sync push failed');
			scheduleReconnect();
			return false;
		}
	};

	const reloadProjectSnapshot = async (reason: string): Promise<boolean> => {
		if (stopped) return false;
		if (inFlightReload) {
			return inFlightReload;
		}

		inFlightReload = (async () => {
			setStatus('connecting', `Reloading snapshot (${reason})...`);
			try {
				const response = await fetch(projectURL);
				if (!response.ok) {
					throw new Error(`reload failed: ${response.status}`);
				}
				const payload = (await response.json()) as unknown;
				const document = coerceProjectResponse(payload);
				if (!document) {
					throw new Error('reload failed: invalid response payload');
				}
				applyRemoteSnapshot(document, document.version, document);
				setStatus('connected', `Reloaded snapshot (v${lastVersion})`);
				return true;
			} catch (error) {
				setStatus('offline', error instanceof Error ? error.message : 'reload failed');
				scheduleReconnect();
				return false;
			} finally {
				inFlightReload = null;
			}
		})();

		return inFlightReload;
	};

	const flushPendingOps = async () => {
		if (stopped || !localSyncReady) return;
		if (inFlightPush) {
			pendingPush = true;
			return;
		}
		inFlightPush = true;

		try {
			while (!stopped) {
				const op = takeNextOperation();
				if (!op) break;
				const ok = await executeOperation(op);
				if (!ok) {
					restoreOperation(op);
					break;
				}
			}
		} finally {
			inFlightPush = false;
			if (pendingPush) {
				pendingPush = false;
				schedulePush(80);
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
			if (!isObjectRecord(payload)) return;
			const sender = typeof payload.clientId === 'string' ? payload.clientId : undefined;
			const response = coerceProjectResponse(payload);
			if (!response) return;
			if (sender && sender === clientID) {
				lastVersion = Math.max(lastVersion, response.version);
				return;
			}
			if (response.version <= lastVersion) return;
			applyRemoteSnapshot(response, response.version, response);
			setStatus('connected', `Received snapshot (v${lastVersion})`);
		});

		const handleEntityEvent = (eventName: string) => {
			es.addEventListener(eventName, (event) => {
				if (stopped) return;
				let payload: unknown;
				try {
					payload = JSON.parse((event as MessageEvent).data);
				} catch {
					return;
				}
				const update = coerceEntityEvent(payload);
				if (!update) return;
				if (update.clientId && update.clientId === clientID) {
					lastVersion = Math.max(lastVersion, update.version);
					return;
				}
				if (update.version > 0 && update.version <= lastVersion) return;
				if (update.version > 0 && update.version > lastVersion + 1) {
					void reloadProjectSnapshot(`stream gap: v${lastVersion} -> v${update.version}`);
					return;
				}

				if (update.entity === 'glyph') {
					if (!update.entityId) return;
					if (update.entityDeleted) {
						applyRemoteGlyphDelete(update.entityId, update.entityVersion, update.version);
					} else {
						const glyph = coerceGlyph(update.payload);
						if (!glyph) return;
						applyRemoteGlyphUpsert(glyph, update.entityVersion, update.version);
					}
				} else if (update.entity === 'syntax') {
					if (!update.entityId) return;
					if (update.entityDeleted) {
						applyRemoteSyntaxDelete(update.entityId, update.entityVersion, update.version);
					} else {
						const syntax = coerceSyntax(update.payload);
						if (!syntax) return;
						applyRemoteSyntaxUpsert(syntax, update.entityVersion, update.version);
					}
				} else if (update.entity === 'metrics') {
					const nextMetrics = coerceMetrics(update.payload);
					if (!nextMetrics) return;
					applyRemoteMetricsUpdate(nextMetrics, update.entityVersion, update.version);
				}

				setStatus('connected', `Received update (v${lastVersion})`);
			});
		};

		handleEntityEvent('glyph_upsert');
		handleEntityEvent('glyph_delete');
		handleEntityEvent('syntax_upsert');
		handleEntityEvent('syntax_delete');
		handleEntityEvent('metrics_update');

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
				const document = coerceProjectResponse(payload);
				if (!document) {
					throw new Error('load failed: invalid response payload');
				}
				applyRemoteSnapshot(document, document.version, document);
				loadedRemote = true;
				setStatus('connected', `Loaded snapshot (v${lastVersion})`);
			}
		} catch (error) {
			setStatus('offline', error instanceof Error ? error.message : 'load failed');
		}

		refreshLocalHashes();
		unsubs.push(glyphs.subscribe(syncLocalGlyphQueue));
		unsubs.push(syntaxes.subscribe(syncLocalSyntaxQueue));
		unsubs.push(metrics.subscribe(syncLocalMetricsQueue));
		localSyncReady = true;

		if (!loadedRemote) {
			queueFullLocalState();
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
