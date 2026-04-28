import type { StoredSession } from './types.ts';

const TOKEN_KEY = 'gomessenger_token';
const USERNAME_KEY = 'gomessenger_username';
const FRIEND_CODE_KEY = 'gomessenger_friend_code';
const ROLE_KEY = 'gomessenger_role';

export const parseJwtPayload = (token: string): Record<string, unknown> => {
	const parts = token.split('.');
	if (parts.length < 2) {
		return {};
	}

	try {
		const normalized = parts[1].replace(/-/g, '+').replace(/_/g, '/');
		const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), '=');
		return JSON.parse(atob(padded)) as Record<string, unknown>;
	} catch {
		return {};
	}
};

export const parseJwtUserId = (token: string) => {
	const payload = parseJwtPayload(token);

	if (typeof payload.userId === 'string') {
		return payload.userId;
	}

	if (typeof payload.user_id === 'string') {
		return payload.user_id;
	}

	if (typeof payload.sub === 'string') {
		return payload.sub;
	}

	return '';
};

export const parseJwtRole = (token: string) => {
	const payload = parseJwtPayload(token);

	if (typeof payload.role === 'string') {
		return payload.role;
	}

	return '';
};

export const loadStoredSession = (): StoredSession | null => {
	const token = localStorage.getItem(TOKEN_KEY);
	const username = localStorage.getItem(USERNAME_KEY);
	const friendCode = localStorage.getItem(FRIEND_CODE_KEY) ?? '';
	const storedRole = localStorage.getItem(ROLE_KEY) ?? '';

	if (!token || !username) {
		return null;
	}

	const userId = parseJwtUserId(token);
	const role = parseJwtRole(token) || storedRole;

	return {
		token,
		username,
		userId,
		friendCode,
		role,
	};
};

export const saveStoredSession = (session: Pick<StoredSession, 'token' | 'username' | 'friendCode'> & { role?: string }) => {
	localStorage.setItem(TOKEN_KEY, session.token);
	localStorage.setItem(USERNAME_KEY, session.username);
	if (session.friendCode) {
		localStorage.setItem(FRIEND_CODE_KEY, session.friendCode);
	} else {
		localStorage.removeItem(FRIEND_CODE_KEY);
	}

	const role = parseJwtRole(session.token) || session.role || '';
	if (role) {
		localStorage.setItem(ROLE_KEY, role);
	} else {
		localStorage.removeItem(ROLE_KEY);
	}
};

export const clearStoredSession = () => {
	localStorage.removeItem(TOKEN_KEY);
	localStorage.removeItem(USERNAME_KEY);
	localStorage.removeItem(FRIEND_CODE_KEY);
	localStorage.removeItem(ROLE_KEY);
};
