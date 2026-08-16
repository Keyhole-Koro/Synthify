import { createRPCClient } from '@/lib/connect';
import { ChatService } from '@/gen/proto/synthify/app/v1/chat_pb';
import { ChatRole } from '@/gen/proto/synthify/app/v1/chat_pb';

const chatClient = createRPCClient(ChatService);

export interface ChatHistoryEntry {
  role: 'user' | 'assistant';
  text: string;
}

export interface ChatContextPaper {
  paperId: string;
  title: string;
  description: string;
  /** Plain text: paper content may be HTML, ContentNode[] or live JSX. */
  content: string;
}

export interface PostChatTurnArgs {
  workspaceId: string;
  paperId: string;
  /** Empty starts a new chat; pass the returned id to continue one. */
  chatId?: string;
  history: ChatHistoryEntry[];
  candidates: ChatContextPaper[];
  pinnedContextIds?: string[];
  autoSelectContext?: boolean;
}

/**
 * Triggers one turn. Returns as soon as the turn is registered, not when the
 * answer is ready — the answer arrives through the Firestore turn document,
 * which useChatTurn subscribes to.
 */
export async function postChatTurn(args: PostChatTurnArgs) {
  const res = await chatClient.postChatTurn({
    workspaceId: args.workspaceId,
    paperId: args.paperId,
    chatId: args.chatId ?? '',
    history: args.history.map((h) => ({
      role: h.role === 'assistant' ? ChatRole.ASSISTANT : ChatRole.USER,
      text: h.text,
    })),
    candidates: args.candidates,
    pinnedContextIds: args.pinnedContextIds ?? [],
    autoSelectContext: args.autoSelectContext ?? false,
  });
  return { chatId: res.chatId, turnId: res.turnId };
}
