import { createRPCClient } from '@/lib/connect';
import {
  WorkspaceChatService,
  WorkspaceChatMessageStatus,
  type WorkspaceChatConversation,
  type WorkspaceChatMessage,
  type WorkspaceChatSource,
} from '@/gen/proto/synthify/app/v1/workspace_chat_pb';

export type {
  WorkspaceChatConversation,
  WorkspaceChatMessage,
  WorkspaceChatSource,
};
export { WorkspaceChatMessageStatus };

const client = createRPCClient(WorkspaceChatService);

/**
 * Lists existing chat conversations for a workspace and checks whether answerable document sources exist.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Object with array of conversations and boolean `hasAnswerableSources`.
 */
export async function listChatConversations(workspaceId: string) {
  const res = await client.listWorkspaceChatConversations({ workspaceId });
  return {
    conversations: res.conversations,
    // Whether sources exist is decided strictly by the server matching retrieval conditions.
    hasAnswerableSources: res.hasAnswerableSources,
  };
}

/**
 * Retrieves the sequential message history of a chat conversation.
 *
 * @param workspaceId - Workspace identifier.
 * @param conversationId - Conversation thread identifier.
 * @returns Array of WorkspaceChatMessage records.
 */
export async function listChatMessages(workspaceId: string, conversationId: string) {
  const res = await client.listWorkspaceChatMessages({ workspaceId, conversationId });
  return res.messages;
}

/**
 * Sends a user question to the workspace chat assistant and awaits the grounded assistant reply.
 *
 * @param workspaceId - Workspace identifier.
 * @param conversationId - Optional conversation ID, or empty string to start a new thread.
 * @param text - The user's query text.
 * @returns Conversation ID, persisted user message, and assistant reply.
 */
export async function sendChatMessage(
  workspaceId: string,
  conversationId: string,
  text: string,
) {
  const res = await client.sendWorkspaceChatMessage({ workspaceId, conversationId, text });
  return {
    conversationId: res.conversationId,
    userMessage: res.userMessage!,
    assistantMessage: res.assistantMessage!,
  };
}
