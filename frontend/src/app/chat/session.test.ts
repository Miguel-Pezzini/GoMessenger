import assert from 'node:assert/strict';
import test from 'node:test';

import { clearStoredSession, loadStoredSession, parseJwtRole, parseJwtUserId, saveStoredSession } from './session.ts';

class MemoryStorage {
  private values = new Map<string, string>();

  getItem(key: string) {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string) {
    this.values.set(key, value);
  }

  removeItem(key: string) {
    this.values.delete(key);
  }
}

const makeToken = (payload: Record<string, unknown>) => {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const body = Buffer.from(JSON.stringify(payload)).toString('base64url');
  return `${header}.${body}.signature`;
};

test('parses user id and role from jwt payload', () => {
  const token = makeToken({ userId: 'user-1', role: 'ADMIN' });

  assert.equal(parseJwtUserId(token), 'user-1');
  assert.equal(parseJwtRole(token), 'ADMIN');
});

test('loads stored session with role from jwt as source of truth', () => {
  globalThis.localStorage = new MemoryStorage() as Storage;
  const token = makeToken({ userId: 'admin-1', role: 'ADMIN' });

  saveStoredSession({
    token,
    username: 'admin',
    friendCode: 'FRIEND123',
    role: 'USER',
  });

  assert.deepEqual(loadStoredSession(), {
    token,
    username: 'admin',
    userId: 'admin-1',
    friendCode: 'FRIEND123',
    role: 'ADMIN',
  });

  clearStoredSession();
  assert.equal(loadStoredSession(), null);
});
