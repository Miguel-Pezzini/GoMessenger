import assert from 'node:assert/strict';
import test from 'node:test';

import {
  addOptimisticMessage,
  collectMessagesNeedingSeen,
  createChatState,
  getConversationMessages,
  getConversationUnreadCount,
  markMessagesLocallyRead,
  updateViewedStatus,
  upsertPersistedMessage,
} from './chatState.ts';

test('reconciles an optimistic sender message with the persisted websocket echo', () => {
  const state = createChatState();
  const optimistic = addOptimisticMessage(state, {
    senderId: 'user-a',
    receiverId: 'user-b',
    content: 'hello there',
    timestamp: 1_712_000_000_000,
  });

  upsertPersistedMessage(
    state,
    {
      id: 'msg-1',
      sender_id: 'user-a',
      receiver_id: 'user-b',
      content: 'hello there',
      timestamp: 1_712_000_001,
      viewed_status: 'sent',
    },
    'user-a',
    'realtime'
  );

  const messages = getConversationMessages(state, 'user-b');
  assert.equal(messages.length, 1);
  assert.equal(messages[0].id, 'msg-1');
  assert.equal(messages[0].isOptimistic, false);
  assert.equal(state.messagesById[optimistic.id], undefined);
});

test('keeps viewed status monotonic as delivery and seen events arrive', () => {
  const state = createChatState();

  upsertPersistedMessage(
    state,
    {
      id: 'msg-2',
      sender_id: 'user-a',
      receiver_id: 'user-b',
      content: 'status change',
      timestamp: 1_712_000_100,
      viewed_status: 'sent',
    },
    'user-a',
    'history'
  );

  assert.equal(updateViewedStatus(state, 'msg-2', 'delivered')?.viewedStatus, 'delivered');
  assert.equal(updateViewedStatus(state, 'msg-2', 'seen')?.viewedStatus, 'seen');
  assert.equal(updateViewedStatus(state, 'msg-2', 'delivered')?.viewedStatus, 'seen');
});

test('tracks unread incoming messages separately from read acknowledgements', () => {
  const state = createChatState();

  upsertPersistedMessage(
    state,
    {
      id: 'msg-3',
      sender_id: 'user-b',
      receiver_id: 'user-a',
      content: 'hey',
      timestamp: 1_712_000_200,
      viewed_status: 'sent',
    },
    'user-a',
    'realtime'
  );

  assert.equal(getConversationUnreadCount(state, 'user-b'), 1);
  assert.deepEqual(collectMessagesNeedingSeen(state, 'user-b', 'user-a'), ['msg-3']);

  markMessagesLocallyRead(state, ['msg-3']);
  updateViewedStatus(state, 'msg-3', 'seen');

  assert.equal(getConversationUnreadCount(state, 'user-b'), 0);
  assert.equal(getConversationMessages(state, 'user-b')[0].viewedStatus, 'seen');
});
