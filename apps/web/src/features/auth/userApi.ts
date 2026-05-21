import { createRPCClient } from '@/lib/connect';
import { UserService } from '@/gen/proto/synthify/app/v1/user_pb';

const client = createRPCClient(UserService);

// Firebase サインインに成功するたびに呼ぶ。サーバ側で users / accounts を
// 必要なら作成し、初回ならサインアップクレジットを付与する。冪等。
export async function signInUser() {
  return await client.signInUser({});
}
