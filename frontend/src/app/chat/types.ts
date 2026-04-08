export type AuthMode = 'login' | 'register';

export type ViewedStatus = 'sent' | 'delivered' | 'seen';

export interface Friend {
  id: string;
  userId: string;
  friendId: string;
  createdAt: string;
}

export interface FriendRequest {
  id: string;
  senderId: string;
  receiverId: string;
  createdAt: string;
}

export interface PresenceResponse {
  user_id: string;
  status: 'online' | 'offline' | string;
  last_seen?: string | null;
  current_chat_id?: string;
}

export interface ChatMessageResponse {
  id: string;
  sender_id: string;
  receiver_id: string;
  content: string;
  created_at?: string;
  timestamp?: string | number;
  viewed_status?: string;
}

export interface ConversationResponse {
  messages: ChatMessageResponse[];
  has_more: boolean;
}

export interface RealtimeEventPayload {
  actor_user_id?: string;
  current_chat_id?: string;
  message_id?: string;
  viewed_status?: string;
  occurred_at?: string;
}

export interface RealtimeEventEnvelope {
  type: string;
  payload?: RealtimeEventPayload;
}

export interface ConversationMessage {
  id: string;
  senderId: string;
  receiverId: string;
  content: string;
  timestamp: number;
  viewedStatus: ViewedStatus;
  isMine: boolean;
  isOptimistic: boolean;
  isUnreadIncoming: boolean;
  clientOrder: number;
}

export interface ConversationState {
  hasMore: boolean;
  isLoading: boolean;
  hasLoaded: boolean;
}

export interface StoredSession {
  token: string;
  username: string;
  userId: string;
}

export interface ContactListItem {
  id: string;
  name: string;
  avatar: string;
  lastMessage: string;
  timestamp: string;
  unreadCount: number;
  online: boolean;
  status: string;
  isTyping: boolean;
  isCurrentChat: boolean;
}
