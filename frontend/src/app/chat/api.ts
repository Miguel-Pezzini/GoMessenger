import type {
  AuthMode,
  ChatMessageResponse,
  ConversationResponse,
  Friend,
  FriendRequest,
  PresenceResponse,
} from './types.ts';

export const API_BASE_URL = 'http://localhost:8080';
export const WS_URL = 'ws://localhost:8080/ws';

type ApiClientOptions = {
  getToken: () => string | null;
  onUnauthorized: () => void;
};

type AuthRequestResult = {
  response: Response;
  payload: unknown;
};

const createMessageFromPayload = (payload: unknown, response: Response, fallback: string) => {
  if (
    payload &&
    typeof payload === 'object' &&
    ('error' in payload || 'message' in payload)
  ) {
    const message = String(
      (payload as { error?: string; message?: string }).error ||
        (payload as { message?: string }).message ||
        fallback
    );

    return message;
  }

  return `${fallback} (${response.status})`;
};

export const authenticate = async (params: {
  mode: AuthMode;
  username: string;
  password: string;
}) => {
  const endpoint = params.mode === 'login' ? '/auth/login' : '/auth/register';
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      username: params.username,
      password: params.password,
    }),
  });

  const payload = await response.json().catch(() => ({}));

  if (!response.ok) {
    throw new Error(createMessageFromPayload(payload, response, 'Authentication failed.'));
  }

  return payload as { token?: string; accessToken?: string; jwt?: string };
};

export const createApiClient = (options: ApiClientOptions) => {
  const authRequest = async (path: string, init: RequestInit = {}): Promise<AuthRequestResult> => {
    const token = options.getToken();
    if (!token) {
      throw new Error('Missing session token. Please sign in again.');
    }

    const headers = new Headers(init.headers);
    headers.set('Authorization', `Bearer ${token}`);

    if (init.body && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }

    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers,
    });

    const payload = response.status === 204 ? null : await response.json().catch(() => null);

    if (response.status === 401) {
      options.onUnauthorized();
    }

    return { response, payload };
  };

  const authFetch = async <T>(path: string, init: RequestInit = {}, fallback = 'Request failed.') => {
    const { response, payload } = await authRequest(path, init);

    if (!response.ok) {
      throw new Error(createMessageFromPayload(payload, response, fallback));
    }

    return payload as T;
  };

  return {
    fetchFriends: () => authFetch<Friend[]>('/friends', {}, 'Failed to load friends.'),
    fetchPendingRequests: () =>
      authFetch<FriendRequest[]>('/friends/requests/pending', {}, 'Failed to load pending requests.'),
    fetchConversation: (contactId: string, before?: string) => {
      const searchParams = new URLSearchParams();
      if (before) {
        searchParams.set('before', before);
      }

      const query = searchParams.size > 0 ? `?${searchParams.toString()}` : '';

      return authFetch<ConversationResponse>(
        `/messages/${encodeURIComponent(contactId)}${query}`,
        {},
        'Failed to load messages.'
      );
    },
    fetchPresence: async (userId: string) => {
      const { response, payload } = await authRequest(`/presence/${encodeURIComponent(userId)}`);

      if (response.status === 404) {
        return null;
      }

      if (!response.ok) {
        throw new Error(`Failed to load presence for ${userId}.`);
      }

      return payload as PresenceResponse;
    },
    sendFriendRequest: (receiverId: string) =>
      authFetch<FriendRequest>(
        '/friends/requests',
        {
          method: 'POST',
          body: JSON.stringify({ receiverId }),
        },
        'Failed to send friend request.'
      ),
    acceptFriendRequest: (requestId: string) =>
      authFetch<{ accepted: boolean }>(
        `/friends/requests/${encodeURIComponent(requestId)}/accept`,
        { method: 'POST' },
        'Failed to accept friend request.'
      ),
    declineFriendRequest: (requestId: string) =>
      authFetch<null>(
        `/friends/requests/${encodeURIComponent(requestId)}/decline`,
        { method: 'DELETE' },
        'Failed to decline friend request.'
      ),
    removeFriend: (friendId: string) =>
      authFetch<null>(
        `/friends/${encodeURIComponent(friendId)}`,
        { method: 'DELETE' },
        'Failed to remove friend.'
      ),
  };
};

export const isPersistedChatMessage = (value: unknown): value is ChatMessageResponse => {
  if (!value || typeof value !== 'object') {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.id === 'string' &&
    typeof candidate.sender_id === 'string' &&
    typeof candidate.receiver_id === 'string' &&
    typeof candidate.content === 'string'
  );
};
