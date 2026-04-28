import { API_BASE_URL } from '../chat/api.ts';

export interface AdminLogEvent {
  stream_id: string;
  event_id: string;
  event_type: string;
  category: string;
  service: string;
  actor_user_id?: string;
  target_user_id?: string;
  entity_type?: string;
  entity_id?: string;
  occurred_at: string;
  status: string;
  message: string;
  metadata?: Record<string, unknown>;
}

export interface AdminLogFilters {
  limit: number;
  service: string;
  category: string;
  status: string;
  eventType: string;
  actorUserId: string;
  query: string;
}

export interface ActiveUser {
  user_id: string;
  username?: string;
  status: string;
  current_chat_id?: string;
  last_seen?: string | null;
}

export interface ActiveUsersResponse {
  users: ActiveUser[];
  count: number;
}

const createAdminError = async (response: Response, fallback: string) => {
  const payload = await response.json().catch(() => null);
  if (payload && typeof payload === 'object') {
    const message = (payload as { error?: string; message?: string }).error || (payload as { message?: string }).message;
    if (message) {
      return new Error(String(message));
    }
  }
  return new Error(`${fallback} (${response.status})`);
};

const authHeaders = (token: string) => ({
  Authorization: `Bearer ${token}`,
});

export const fetchAdminLogs = async (token: string, filters: AdminLogFilters) => {
  const params = new URLSearchParams();
  params.set('limit', String(filters.limit || 100));
  if (filters.service) params.set('service', filters.service);
  if (filters.category) params.set('category', filters.category);
  if (filters.status) params.set('status', filters.status);
  if (filters.eventType) params.set('event_type', filters.eventType);
  if (filters.actorUserId) params.set('actor_user_id', filters.actorUserId);
  if (filters.query) params.set('q', filters.query);

  const response = await fetch(`${API_BASE_URL}/admin/logs?${params.toString()}`, {
    headers: authHeaders(token),
  });

  if (!response.ok) {
    throw await createAdminError(response, 'Failed to load admin logs.');
  }

  return (await response.json()) as AdminLogEvent[];
};

export const fetchActiveUsers = async (token: string, limit = 100) => {
  const response = await fetch(`${API_BASE_URL}/admin/presence/active?limit=${encodeURIComponent(String(limit))}`, {
    headers: authHeaders(token),
  });

  if (!response.ok) {
    throw await createAdminError(response, 'Failed to load active users.');
  }

  return (await response.json()) as ActiveUsersResponse;
};

export const createAdminLogsSocket = (token: string) => {
  const wsBase = API_BASE_URL.replace(/^https?:/, (scheme) => (scheme === 'https:' ? 'wss:' : 'ws:'));
  return new WebSocket(`${wsBase}/admin/logs/ws?token=${encodeURIComponent(token)}`);
};
