import type { PresenceResponse, ViewedStatus } from './types.ts';

const timeFormatter = new Intl.DateTimeFormat('en-US', {
  hour: 'numeric',
  minute: '2-digit',
});

const dateFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
});

const dateTimeFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
});

const isSameDay = (left: Date, right: Date) => {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
};

export const normalizeTimestampMs = (value?: string | number | null) => {
  if (typeof value === 'number') {
    return value < 1_000_000_000_000 ? value * 1000 : value;
  }

  if (typeof value === 'string' && value) {
    const numericValue = Number(value);
    if (Number.isFinite(numericValue)) {
      return normalizeTimestampMs(numericValue);
    }

    const parsed = Date.parse(value);
    if (!Number.isNaN(parsed)) {
      return parsed;
    }
  }

  return Date.now();
};

export const formatMessageTime = (timestamp: number) => {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return '';
  }

  return timeFormatter.format(date);
};

export const formatConversationTimestamp = (timestamp: number) => {
  const date = new Date(timestamp);
  const now = new Date();

  if (Number.isNaN(date.getTime())) {
    return '';
  }

  if (isSameDay(date, now)) {
    return timeFormatter.format(date);
  }

  return dateFormatter.format(date);
};

export const formatPresenceText = (presence: PresenceResponse | null | undefined, currentUserId: string) => {
  if (!presence) {
    return 'Checking presence...';
  }

  if (presence.status === 'online' && presence.current_chat_id === currentUserId) {
    return 'Active in this chat';
  }

  if (presence.status === 'online') {
    return 'Online';
  }

  if (presence.last_seen) {
    return `Last seen ${dateTimeFormatter.format(new Date(presence.last_seen))}`;
  }

  return 'Offline';
};

export const formatStatusLabel = (status: ViewedStatus) => {
  switch (status) {
    case 'seen':
      return 'Seen';
    case 'delivered':
      return 'Delivered';
    default:
      return 'Sent';
  }
};
