import type { StoredSession } from './types.ts';

const TOKEN_KEY = 'gomessenger_token';
const USERNAME_KEY = 'gomessenger_username';

export const parseJwtUserId = (token: string) => {
  const parts = token.split('.');
  if (parts.length < 2) {
    return '';
  }

  try {
    const normalized = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), '=');
    const payload = JSON.parse(atob(padded)) as Record<string, unknown>;

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
  } catch {
    return '';
  }
};

export const loadStoredSession = (): StoredSession | null => {
  const token = localStorage.getItem(TOKEN_KEY);
  const username = localStorage.getItem(USERNAME_KEY);

  if (!token || !username) {
    return null;
  }

  const userId = parseJwtUserId(token);

  return {
    token,
    username,
    userId,
  };
};

export const saveStoredSession = (session: Pick<StoredSession, 'token' | 'username'>) => {
  localStorage.setItem(TOKEN_KEY, session.token);
  localStorage.setItem(USERNAME_KEY, session.username);
};

export const clearStoredSession = () => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USERNAME_KEY);
};
