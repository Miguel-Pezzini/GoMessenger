import { normalizeTimestampMs } from './format.ts';
import type { ChatMessageResponse, ConversationMessage, ViewedStatus } from './types.ts';

export interface ChatState {
  messagesById: Record<string, ConversationMessage>;
  conversationMessageIds: Record<string, string[]>;
  localSequence: number;
}

type UpsertSource = 'history' | 'realtime';

const viewedStatusRank = (status: ViewedStatus) => {
  switch (status) {
    case 'delivered':
      return 1;
    case 'seen':
      return 2;
    default:
      return 0;
  }
};

export const normalizeViewedStatus = (status?: string | null): ViewedStatus => {
  if (status === 'delivered') {
    return 'delivered';
  }

  if (status === 'seen') {
    return 'seen';
  }

  return 'sent';
};

const chooseHigherStatus = (left: ViewedStatus, right: ViewedStatus) => {
  return viewedStatusRank(left) >= viewedStatusRank(right) ? left : right;
};

const nextSequence = (state: ChatState) => {
  state.localSequence += 1;
  return state.localSequence;
};

const ensureConversation = (state: ChatState, contactId: string) => {
  if (!state.conversationMessageIds[contactId]) {
    state.conversationMessageIds[contactId] = [];
  }

  return state.conversationMessageIds[contactId];
};

const sortConversationIds = (state: ChatState, contactId: string) => {
  state.conversationMessageIds[contactId].sort((leftId, rightId) => {
    const left = state.messagesById[leftId];
    const right = state.messagesById[rightId];

    if (left.timestamp !== right.timestamp) {
      return left.timestamp - right.timestamp;
    }

    return left.clientOrder - right.clientOrder;
  });
};

const replaceConversationId = (state: ChatState, contactId: string, fromId: string, toId: string) => {
  const ids = ensureConversation(state, contactId);
  const index = ids.indexOf(fromId);

  if (index >= 0) {
    ids[index] = toId;
  }
};

const getConversationId = (
  senderId: string,
  receiverId: string,
  currentUserId: string
) => {
  return senderId === currentUserId ? receiverId : senderId;
};

const findOptimisticMatchId = (
  state: ChatState,
  currentUserId: string,
  message: ChatMessageResponse,
  conversationId: string,
  timestamp: number
) => {
  const ids = ensureConversation(state, conversationId);

  return ids.find((id) => {
    const candidate = state.messagesById[id];
    if (!candidate || !candidate.isOptimistic || !candidate.isMine) {
      return false;
    }

    if (
      candidate.senderId !== currentUserId ||
      candidate.receiverId !== message.receiver_id ||
      candidate.content !== message.content
    ) {
      return false;
    }

    return Math.abs(candidate.timestamp - timestamp) < 120000;
  });
};

export const createChatState = (): ChatState => ({
  messagesById: {},
  conversationMessageIds: {},
  localSequence: 0,
});

export const addOptimisticMessage = (
  state: ChatState,
  params: {
    senderId: string;
    receiverId: string;
    content: string;
    timestamp?: number;
  }
) => {
  const message: ConversationMessage = {
    id: `local-${Date.now()}-${nextSequence(state)}`,
    senderId: params.senderId,
    receiverId: params.receiverId,
    content: params.content,
    timestamp: params.timestamp ?? Date.now(),
    viewedStatus: 'sent',
    isMine: true,
    isOptimistic: true,
    isUnreadIncoming: false,
    clientOrder: state.localSequence,
  };

  state.messagesById[message.id] = message;
  ensureConversation(state, params.receiverId).push(message.id);
  sortConversationIds(state, params.receiverId);

  return message;
};

export const upsertPersistedMessage = (
  state: ChatState,
  message: ChatMessageResponse,
  currentUserId: string,
  source: UpsertSource
) => {
  const timestamp = normalizeTimestampMs(message.created_at ?? message.timestamp);
  const conversationId = getConversationId(message.sender_id, message.receiver_id, currentUserId);
  const optimisticMatchId = findOptimisticMatchId(state, currentUserId, message, conversationId, timestamp);

  let inheritedMessage: ConversationMessage | undefined;
  if (optimisticMatchId && optimisticMatchId !== message.id) {
    inheritedMessage = state.messagesById[optimisticMatchId];
    delete state.messagesById[optimisticMatchId];
    replaceConversationId(state, conversationId, optimisticMatchId, message.id);
  }

  const existing = state.messagesById[message.id];
  const nextMessage: ConversationMessage = {
    id: message.id,
    senderId: message.sender_id,
    receiverId: message.receiver_id,
    content: message.content,
    timestamp,
    viewedStatus: chooseHigherStatus(
      existing?.viewedStatus ?? inheritedMessage?.viewedStatus ?? 'sent',
      normalizeViewedStatus(message.viewed_status)
    ),
    isMine: message.sender_id === currentUserId,
    isOptimistic: false,
    isUnreadIncoming:
      existing?.isUnreadIncoming ??
      inheritedMessage?.isUnreadIncoming ??
      (source === 'realtime' && message.sender_id !== currentUserId),
    clientOrder: existing?.clientOrder ?? inheritedMessage?.clientOrder ?? nextSequence(state),
  };

  state.messagesById[message.id] = nextMessage;

  const ids = ensureConversation(state, conversationId);
  if (!ids.includes(message.id)) {
    ids.push(message.id);
  }

  sortConversationIds(state, conversationId);

  return {
    conversationId,
    messageId: message.id,
    isIncoming: message.sender_id !== currentUserId,
  };
};

export const updateViewedStatus = (state: ChatState, messageId: string, status: ViewedStatus) => {
  const message = state.messagesById[messageId];
  if (!message) {
    return null;
  }

  message.viewedStatus = chooseHigherStatus(message.viewedStatus, normalizeViewedStatus(status));
  return message;
};

export const markMessagesLocallyRead = (state: ChatState, messageIds: string[]) => {
  for (const messageId of messageIds) {
    const message = state.messagesById[messageId];
    if (!message) {
      continue;
    }

    message.isUnreadIncoming = false;
  }
};

export const getConversationMessages = (state: ChatState, contactId: string) => {
  const ids = ensureConversation(state, contactId);
  return ids.map((id) => state.messagesById[id]);
};

export const getConversationLastMessage = (state: ChatState, contactId: string) => {
  const ids = ensureConversation(state, contactId);
  const lastId = ids[ids.length - 1];
  return lastId ? state.messagesById[lastId] : null;
};

export const getConversationUnreadCount = (state: ChatState, contactId: string) => {
  return getConversationMessages(state, contactId).filter((message) => message.isUnreadIncoming).length;
};

export const getOldestPersistedMessageId = (state: ChatState, contactId: string) => {
  return getConversationMessages(state, contactId).find((message) => !message.isOptimistic)?.id ?? null;
};

export const collectMessagesNeedingSeen = (state: ChatState, contactId: string, currentUserId: string) => {
  return getConversationMessages(state, contactId)
    .filter((message) => message.senderId !== currentUserId && message.viewedStatus !== 'seen')
    .map((message) => message.id);
};
