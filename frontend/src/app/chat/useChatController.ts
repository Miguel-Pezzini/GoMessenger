import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';

import { API_BASE_URL, WS_URL, authenticate, createApiClient, isPersistedChatMessage } from './api.ts';
import {
  addOptimisticMessage,
  collectMessagesNeedingSeen,
  createChatState,
  getConversationLastMessage,
  getConversationMessages,
  getConversationUnreadCount,
  getOldestPersistedMessageId,
  markMessagesLocallyRead,
  updateViewedStatus,
  upsertPersistedMessage,
} from './chatState.ts';
import { formatConversationTimestamp, formatPresenceText } from './format.ts';
import { clearStoredSession, loadStoredSession, parseJwtUserId, saveStoredSession } from './session.ts';
import type {
  AuthMode,
  ContactListItem,
  ConversationState,
  Friend,
  FriendRequest,
  PresenceResponse,
  RealtimeEventEnvelope,
} from './types.ts';

type ConnectionState = 'offline' | 'connecting' | 'connected' | 'reconnecting';

const friendEventTypes = new Set([
  'friend_request_received',
  'friend_request_accepted',
  'friend_request_declined',
  'friend_removed',
]);

const typingIndicatorTimeoutMs = 3500;
const presencePollIntervalMs = 10000;

export const createDefaultPresence = (userId: string): PresenceResponse => ({
  user_id: userId,
  status: 'offline',
  last_seen: null,
  current_chat_id: '',
});

export const useChatController = () => {
  const authMode = ref<AuthMode>('login');
  const isAuthenticated = ref(false);
  const isFriendsLoading = ref(false);
  const currentUser = ref('');
  const currentUserId = ref('');
  const sessionToken = ref('');
  const actionError = ref('');
  const actionSuccess = ref('');
  const connectionState = ref<ConnectionState>('offline');
  const friends = ref<Friend[]>([]);
  const pendingRequests = ref<FriendRequest[]>([]);
  const selectedContactId = ref<string | null>(null);
  const conversationState = ref<Record<string, ConversationState>>({});
  const presenceByUserId = ref<Record<string, PresenceResponse>>({});
  const typingByContactId = ref<Record<string, boolean>>({});
  const chatState = reactive(createChatState());

  const deliveredAcknowledgements = new Set<string>();
  const seenAcknowledgements = new Set<string>();
  const typingTimers = new Map<string, number>();

  let ws: WebSocket | null = null;
  let manualDisconnect = false;
  let reconnectTimer: number | null = null;
  let reconnectAttempt = 0;
  let presencePollTimer: number | null = null;
  let localTypingTargetId: string | null = null;
  let activeChatReportedId: string | null = null;

  const apiClient = createApiClient({
    getToken: () => sessionToken.value || null,
    onUnauthorized: () => {
      handleLogout();
    },
  });

  const getConversationViewState = (contactId: string): ConversationState => {
    if (!conversationState.value[contactId]) {
      conversationState.value[contactId] = {
        hasMore: true,
        isLoading: false,
        hasLoaded: false,
      };
    }

    return conversationState.value[contactId];
  };

  const selectedConversationState = computed<ConversationState>(() => {
    if (!selectedContactId.value) {
      return {
        hasMore: false,
        isLoading: false,
        hasLoaded: false,
      };
    }

    return getConversationViewState(selectedContactId.value);
  });

  const currentMessages = computed(() => {
    if (!selectedContactId.value) {
      return [];
    }

    return getConversationMessages(chatState, selectedContactId.value);
  });

  const selectedPresence = computed(() => {
    if (!selectedContactId.value) {
      return null;
    }

    return presenceByUserId.value[selectedContactId.value] ?? createDefaultPresence(selectedContactId.value);
  });

  const isPeerTyping = computed(() => {
    if (!selectedContactId.value) {
      return false;
    }

    return Boolean(typingByContactId.value[selectedContactId.value]);
  });

  const selectedContact = computed(() => contacts.value.find((contact) => contact.id === selectedContactId.value));

  const selectedContactStatus = computed(() => {
    if (isPeerTyping.value) {
      return 'typing...';
    }

    return formatPresenceText(selectedPresence.value, currentUserId.value);
  });

  const composerDisabled = computed(() => {
    return !selectedContactId.value || connectionState.value !== 'connected';
  });

  const composerPlaceholder = computed(() => {
    if (!selectedContactId.value) {
      return 'Select a friend to start chatting';
    }

    if (connectionState.value === 'reconnecting' || connectionState.value === 'connecting') {
      return 'Reconnecting to chat...';
    }

    return 'Type a message';
  });

  function isConversationVisible() {
    return Boolean(selectedContactId.value) && document.visibilityState === 'visible';
  }

  function resetObjectValues<T>(target: Record<string, T>) {
    for (const key of Object.keys(target)) {
      delete target[key];
    }
  }

  function clearReconnectTimer() {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function stopPresencePolling() {
    if (presencePollTimer !== null) {
      window.clearInterval(presencePollTimer);
      presencePollTimer = null;
    }
  }

  function stopTypingIndicator(contactId: string) {
    const timerId = typingTimers.get(contactId);
    if (timerId) {
      window.clearTimeout(timerId);
      typingTimers.delete(contactId);
    }

    if (typingByContactId.value[contactId]) {
      typingByContactId.value = {
        ...typingByContactId.value,
        [contactId]: false,
      };
    }
  }

  function startTypingIndicator(contactId: string) {
    stopTypingIndicator(contactId);
    typingByContactId.value = {
      ...typingByContactId.value,
      [contactId]: true,
    };

    const timerId = window.setTimeout(() => {
      stopTypingIndicator(contactId);
    }, typingIndicatorTimeoutMs);

    typingTimers.set(contactId, timerId);
  }

  function defaultPresenceForFriends(nextFriends: Friend[]) {
    const nextPresenceEntries = nextFriends.map((friend) => [
      friend.friendId,
      presenceByUserId.value[friend.friendId] ?? createDefaultPresence(friend.friendId),
    ] as const);

    presenceByUserId.value = Object.fromEntries(nextPresenceEntries);
  }

  function previewForMessage(contactId: string) {
    const lastMessage = getConversationLastMessage(chatState, contactId);

    if (typingByContactId.value[contactId]) {
      return 'typing...';
    }

    if (!lastMessage) {
      const friend = friends.value.find((entry) => entry.friendId === contactId);
      return friend ? 'Friend connection created in backend' : 'Start a conversation';
    }

    const prefix = lastMessage.isMine ? 'You: ' : '';
    return `${prefix}${lastMessage.content}`;
  }

  const contacts = computed<ContactListItem[]>(() => {
    return friends.value.map((friend) => {
      const contactId = friend.friendId;
      const lastMessage = getConversationLastMessage(chatState, contactId);
      const presence = presenceByUserId.value[contactId];

      return {
        id: contactId,
        name: contactId,
        avatar: '',
        lastMessage: previewForMessage(contactId),
        timestamp: lastMessage
          ? formatConversationTimestamp(lastMessage.timestamp)
          : formatConversationTimestamp(Number.isNaN(Date.parse(friend.createdAt)) ? Date.now() : Date.parse(friend.createdAt)),
        unreadCount: getConversationUnreadCount(chatState, contactId),
        online: presence?.status === 'online',
        status: formatPresenceText(presence, currentUserId.value),
        isTyping: Boolean(typingByContactId.value[contactId]),
        isCurrentChat: Boolean(
          presence?.status === 'online' &&
            presence.current_chat_id &&
            presence.current_chat_id === currentUserId.value
        ),
      };
    });
  });

  async function refreshActivePresence(contactId = selectedContactId.value) {
    if (!contactId) {
      return;
    }

    try {
      const presence = await apiClient.fetchPresence(contactId);
      presenceByUserId.value = {
        ...presenceByUserId.value,
        [contactId]: presence ?? createDefaultPresence(contactId),
      };
    } catch (error) {
      console.error(error);
    }
  }

  function startPresencePolling() {
    stopPresencePolling();

    if (!selectedContactId.value || !isAuthenticated.value) {
      return;
    }

    presencePollTimer = window.setInterval(() => {
      void refreshActivePresence();
    }, presencePollIntervalMs);
  }

  function closeActiveChat(contactId = activeChatReportedId ?? selectedContactId.value) {
    if (!contactId) {
      return;
    }

    const sent = sendSocketEnvelope('chat_closed', {
      current_chat_id: contactId,
    });

    if (sent && activeChatReportedId === contactId) {
      activeChatReportedId = null;
    }
  }

  function openActiveChat(contactId = selectedContactId.value) {
    if (!contactId || !selectedContactId.value || contactId !== selectedContactId.value || !isConversationVisible()) {
      return;
    }

    if (activeChatReportedId === contactId) {
      return;
    }

    const sent = sendSocketEnvelope('chat_opened', {
      current_chat_id: contactId,
    });

    if (sent) {
      activeChatReportedId = contactId;
    }
  }

  function sendSocketEnvelope(type: string, payload: Record<string, unknown>) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return false;
    }

    ws.send(
      JSON.stringify({
        type,
        payload,
      })
    );

    return true;
  }

  function scheduleReconnect() {
    clearReconnectTimer();
    connectionState.value = 'reconnecting';

    const delay = Math.min(8000, 750 * 2 ** reconnectAttempt);
    reconnectAttempt += 1;

    reconnectTimer = window.setTimeout(() => {
      if (!sessionToken.value || !isAuthenticated.value) {
        return;
      }

      connectWebSocket(sessionToken.value);
    }, delay);
  }

  function sendDeliveredAcknowledgement(messageId: string, targetUserId: string) {
    if (deliveredAcknowledgements.has(messageId)) {
      return;
    }

    const sent = sendSocketEnvelope('message_delivered', {
      target_user_id: targetUserId,
      current_chat_id: targetUserId,
      message_id: messageId,
    });

    if (sent) {
      deliveredAcknowledgements.add(messageId);
      updateViewedStatus(chatState, messageId, 'delivered');
    }
  }

  async function acknowledgeVisibleConversation() {
    if (!selectedContactId.value || !isConversationVisible() || connectionState.value !== 'connected') {
      return;
    }

    const pendingIds = collectMessagesNeedingSeen(chatState, selectedContactId.value, currentUserId.value).filter(
      (messageId) => !seenAcknowledgements.has(messageId)
    );

    if (pendingIds.length === 0) {
      return;
    }

    for (const messageId of pendingIds) {
      const message = chatState.messagesById[messageId];
      if (!message) {
        continue;
      }

      const sent = sendSocketEnvelope('message_seen', {
        target_user_id: message.senderId,
        current_chat_id: message.senderId,
        message_id: messageId,
      });

      if (!sent) {
        continue;
      }

      seenAcknowledgements.add(messageId);
      markMessagesLocallyRead(chatState, [messageId]);
      updateViewedStatus(chatState, messageId, 'seen');
    }
  }

  function handleRealtimeEvent(event: RealtimeEventEnvelope) {
    if (!event.type) {
      return;
    }

    if (friendEventTypes.has(event.type)) {
      void refreshFriendsData();
      return;
    }

    const payload = event.payload ?? {};

    switch (event.type) {
      case 'typing_started':
        if (payload.actor_user_id && payload.actor_user_id !== currentUserId.value) {
          startTypingIndicator(payload.actor_user_id);
        }
        break;
      case 'typing_stopped':
        if (payload.actor_user_id) {
          stopTypingIndicator(payload.actor_user_id);
        }
        break;
      case 'message_delivered':
        if (payload.message_id) {
          updateViewedStatus(chatState, payload.message_id, 'delivered');
        }
        break;
      case 'message_seen':
        if (payload.message_id) {
          updateViewedStatus(chatState, payload.message_id, 'seen');
        }
        break;
      default:
        break;
    }
  }

  function handlePersistedMessage(rawMessage: unknown) {
    if (!isPersistedChatMessage(rawMessage)) {
      return;
    }

    const result = upsertPersistedMessage(chatState, rawMessage, currentUserId.value, 'realtime');

    if (!result.isIncoming) {
      return;
    }

    stopTypingIndicator(result.conversationId);
    sendDeliveredAcknowledgement(result.messageId, rawMessage.sender_id);

    if (selectedContactId.value === result.conversationId) {
      void nextTick(() => {
        void acknowledgeVisibleConversation();
      });
    }
  }

  function handleSocketMessage(data: string) {
    try {
      const parsed = JSON.parse(data) as unknown;

      if (isPersistedChatMessage(parsed)) {
        handlePersistedMessage(parsed);
        return;
      }

      if (parsed && typeof parsed === 'object' && 'type' in parsed) {
        handleRealtimeEvent(parsed as RealtimeEventEnvelope);
      }
    } catch {
      // Ignore malformed payloads from the websocket.
    }
  }

  function disconnectWebSocket() {
    clearReconnectTimer();

    if (!ws) {
      connectionState.value = 'offline';
      return;
    }

    const socket = ws;
    ws = null;
    socket.close();
    activeChatReportedId = null;
    connectionState.value = 'offline';
  }

  function connectWebSocket(token: string) {
    clearReconnectTimer();
    connectionState.value = reconnectAttempt > 0 ? 'reconnecting' : 'connecting';

    const socket = new WebSocket(`${WS_URL}?token=${encodeURIComponent(token)}`);
    ws = socket;

    socket.addEventListener('open', () => {
      if (ws !== socket) {
        return;
      }

      connectionState.value = 'connected';
      reconnectAttempt = 0;
      activeChatReportedId = null;
      openActiveChat();
      void refreshActivePresence();
      void nextTick(() => {
        void acknowledgeVisibleConversation();
      });
    });

    socket.addEventListener('message', (event: MessageEvent) => {
      if (ws !== socket) {
        return;
      }

      handleSocketMessage(String(event.data));
    });

    socket.addEventListener('close', () => {
      if (ws !== socket) {
        return;
      }

      ws = null;
      activeChatReportedId = null;

      if (manualDisconnect || !isAuthenticated.value || !sessionToken.value) {
        connectionState.value = 'offline';
        return;
      }

      scheduleReconnect();
    });
  }

  async function refreshFriendsData() {
    isFriendsLoading.value = true;

    try {
      const [friendsResponse, pendingResponse] = await Promise.all([
        apiClient.fetchFriends(),
        apiClient.fetchPendingRequests(),
      ]);

      friends.value = friendsResponse;
      pendingRequests.value = pendingResponse;
      defaultPresenceForFriends(friendsResponse);

      if (selectedContactId.value && !friendsResponse.some((friend) => friend.friendId === selectedContactId.value)) {
        selectedContactId.value = null;
      }
    } finally {
      isFriendsLoading.value = false;
    }
  }

  async function loadConversation(contactId: string, before?: string) {
    const state = getConversationViewState(contactId);
    if (state.isLoading) {
      return;
    }

    if (before && !state.hasMore) {
      return;
    }

    state.isLoading = true;
    actionError.value = '';

    try {
      const response = await apiClient.fetchConversation(contactId, before);
      for (const message of response.messages) {
        upsertPersistedMessage(chatState, message, currentUserId.value, 'history');
      }

      state.hasMore = response.has_more;
      state.hasLoaded = true;

      if (selectedContactId.value === contactId) {
        await nextTick();
        await acknowledgeVisibleConversation();
      }
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to load messages.';
    } finally {
      state.isLoading = false;
    }
  }

  async function ensureConversationLoaded(contactId: string) {
    const state = getConversationViewState(contactId);
    if (state.hasLoaded || state.isLoading) {
      return;
    }

    await loadConversation(contactId);
  }

  async function initializeSession() {
    const session = loadStoredSession();
    if (!session) {
      return;
    }

    manualDisconnect = false;
    sessionToken.value = session.token;
    currentUser.value = session.username;
    currentUserId.value = session.userId;
    isAuthenticated.value = true;

    connectWebSocket(session.token);

    try {
      await refreshFriendsData();
      await refreshActivePresence();
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to restore your session.';
    }
  }

  async function handleAuthSubmit(payload: { username: string; password: string; mode: AuthMode }) {
    try {
      const data = await authenticate(payload);

      if (payload.mode === 'register') {
        authMode.value = 'login';
        return {
          success: 'Registration successful! You can sign in now.',
        };
      }

      const token = data.token || data.accessToken || data.jwt || '';
      if (!token) {
        return {
          error: 'Login succeeded but no token was returned.',
        };
      }

      const userId = parseJwtUserId(token);

      saveStoredSession({
        token,
        username: payload.username,
      });

      manualDisconnect = false;
      sessionToken.value = token;
      currentUser.value = payload.username;
      currentUserId.value = userId;
      isAuthenticated.value = true;
      actionError.value = '';
      actionSuccess.value = '';

      connectWebSocket(token);
      await refreshFriendsData();
      await refreshActivePresence();

      return {
        success: '',
      };
    } catch (error) {
      return {
        error: error instanceof Error ? error.message : 'Unable to reach the backend.',
      };
    }
  }

  async function handleSendFriendRequest(receiverId: string) {
    actionError.value = '';
    actionSuccess.value = '';

    try {
      await apiClient.sendFriendRequest(receiverId);
      actionSuccess.value = `Friend request sent to ${receiverId}.`;
      await refreshFriendsData();
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to send friend request.';
    }
  }

  async function handleAcceptRequest(requestId: string) {
    actionError.value = '';
    actionSuccess.value = '';

    try {
      await apiClient.acceptFriendRequest(requestId);
      actionSuccess.value = 'Friend request accepted.';
      await refreshFriendsData();
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to accept friend request.';
    }
  }

  async function handleDeclineRequest(requestId: string) {
    actionError.value = '';
    actionSuccess.value = '';

    try {
      await apiClient.declineFriendRequest(requestId);
      actionSuccess.value = 'Friend request declined.';
      await refreshFriendsData();
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to decline friend request.';
    }
  }

  async function handleRemoveFriend(friendId: string) {
    actionError.value = '';
    actionSuccess.value = '';

    try {
      await apiClient.removeFriend(friendId);
      actionSuccess.value = `Removed ${friendId} from your friends list.`;
      await refreshFriendsData();
    } catch (error) {
      actionError.value = error instanceof Error ? error.message : 'Failed to remove friend.';
    }
  }

  function stopLocalTyping(targetId = localTypingTargetId) {
    if (!targetId) {
      return;
    }

    sendSocketEnvelope('typing_stopped', {
      target_user_id: targetId,
      current_chat_id: targetId,
    });

    if (localTypingTargetId === targetId) {
      localTypingTargetId = null;
    }
  }

  function handleTypingStarted() {
    if (!selectedContactId.value || connectionState.value !== 'connected') {
      return;
    }

    localTypingTargetId = selectedContactId.value;
    sendSocketEnvelope('typing_started', {
      target_user_id: selectedContactId.value,
      current_chat_id: selectedContactId.value,
    });
  }

  function handleTypingStopped() {
    stopLocalTyping();
  }

  function handleSendMessage(text: string) {
    if (!selectedContactId.value || connectionState.value !== 'connected') {
      return;
    }

    const trimmed = text.trim();
    if (!trimmed) {
      return;
    }

    actionError.value = '';

    const sent = sendSocketEnvelope('chat_message', {
      receiver_id: selectedContactId.value,
      content: trimmed,
    });

    if (!sent) {
      actionError.value = 'Chat is reconnecting. Please try again in a moment.';
      return;
    }

    addOptimisticMessage(chatState, {
      senderId: currentUserId.value,
      receiverId: selectedContactId.value,
      content: trimmed,
    });

    getConversationViewState(selectedContactId.value).hasLoaded = true;
    stopLocalTyping(selectedContactId.value);
  }

  function handleSelectContact(contactId: string) {
    if (selectedContactId.value === contactId) {
      return;
    }

    selectedContactId.value = contactId;
  }

  function handleLeaveConversation() {
    selectedContactId.value = null;
  }

  async function handleLoadOlderMessages() {
    if (!selectedContactId.value) {
      return;
    }

    const oldestMessageId = getOldestPersistedMessageId(chatState, selectedContactId.value);

    if (!oldestMessageId) {
      await ensureConversationLoaded(selectedContactId.value);
      return;
    }

    await loadConversation(selectedContactId.value, oldestMessageId);
  }

  function resetChatState() {
    resetObjectValues(chatState.messagesById);
    resetObjectValues(chatState.conversationMessageIds);
    chatState.localSequence = 0;
  }

  function resetTypingState() {
    for (const timerId of typingTimers.values()) {
      window.clearTimeout(timerId);
    }

    typingTimers.clear();
    typingByContactId.value = {};
    localTypingTargetId = null;
  }

  function handleLogout() {
    manualDisconnect = true;
    stopLocalTyping();
    closeActiveChat();
    disconnectWebSocket();
    clearStoredSession();
    stopPresencePolling();
    resetTypingState();
    resetChatState();
    deliveredAcknowledgements.clear();
    seenAcknowledgements.clear();
    currentUser.value = '';
    currentUserId.value = '';
    sessionToken.value = '';
    friends.value = [];
    pendingRequests.value = [];
    selectedContactId.value = null;
    conversationState.value = {};
    presenceByUserId.value = {};
    actionError.value = '';
    actionSuccess.value = '';
    isAuthenticated.value = false;
    authMode.value = 'login';
    reconnectAttempt = 0;
    activeChatReportedId = null;
  }

  function toggleAuthMode() {
    authMode.value = authMode.value === 'login' ? 'register' : 'login';
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible') {
      openActiveChat();
      void refreshActivePresence();
      void nextTick(() => {
        void acknowledgeVisibleConversation();
      });
      return;
    }

    stopLocalTyping();
    closeActiveChat();
  }

  function handlePageHide() {
    stopLocalTyping();
    closeActiveChat();
  }

  watch(selectedContactId, async (nextContactId, previousContactId) => {
    if (previousContactId) {
      stopLocalTyping(previousContactId);
      closeActiveChat(previousContactId);
    }

    if (!nextContactId) {
      stopPresencePolling();
      return;
    }

    await ensureConversationLoaded(nextContactId);
    await refreshActivePresence(nextContactId);
    startPresencePolling();
    openActiveChat(nextContactId);
    await nextTick();
    await acknowledgeVisibleConversation();
  });

  watch(friends, (nextFriends) => {
    if (!selectedContactId.value) {
      return;
    }

    if (!nextFriends.some((friend) => friend.friendId === selectedContactId.value)) {
      selectedContactId.value = null;
    }
  });

  onMounted(() => {
    document.addEventListener('visibilitychange', handleVisibilityChange);
    window.addEventListener('pagehide', handlePageHide);
    void initializeSession();
  });

  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', handleVisibilityChange);
    window.removeEventListener('pagehide', handlePageHide);
    stopLocalTyping();
    closeActiveChat();
    stopPresencePolling();
    disconnectWebSocket();
    resetTypingState();
  });

  return {
    API_BASE_URL,
    actionError,
    actionSuccess,
    authMode,
    composerDisabled,
    composerPlaceholder,
    connectionState,
    contacts,
    currentMessages,
    currentUser,
    currentUserId,
    handleAcceptRequest,
    handleAuthSubmit,
    handleDeclineRequest,
    handleLeaveConversation,
    handleLoadOlderMessages,
    handleLogout,
    handleRemoveFriend,
    handleSelectContact,
    handleSendFriendRequest,
    handleSendMessage,
    handleTypingStarted,
    handleTypingStopped,
    isAuthenticated,
    isFriendsLoading,
    isPeerTyping,
    pendingRequests,
    selectedContact,
    selectedContactId,
    selectedContactStatus,
    selectedConversationState,
    selectedPresence,
    toggleAuthMode,
    acknowledgeVisibleConversation,
  };
};
