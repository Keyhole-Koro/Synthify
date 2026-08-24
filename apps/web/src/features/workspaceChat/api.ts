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

export async function listChatConversations(workspaceId: string) {
  const res = await client.listWorkspaceChatConversations({ workspaceId });
  return {
    conversations: res.conversations,
    // 「質問できるか」の判定はサーバーが持つ。retrieval と同じ条件でなければ
    // ならないので、Firestore の job 状態などから UI 側で推測しない。
    hasAnswerableSources: res.hasAnswerableSources,
  };
}

export async function listChatMessages(workspaceId: string, conversationId: string) {
  const res = await client.listWorkspaceChatMessages({ workspaceId, conversationId });
  return res.messages;
}

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
