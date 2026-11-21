import { createRPCClient } from '@/lib/connect';
import { UserService } from '@/gen/proto/synthify/app/v1/user_pb';

const client = createRPCClient(UserService);

// Firebase サインインに成功するたびに呼ぶ。サーバ側で users / accounts を
// 必要なら作成し、初回ならサインアップクレジットを付与する。冪等。
export async function signInUser() {
  return await client.signInUser({});
}

// 退会。サーバ側で自分のワークスペース・メンバーシップ・アカウント・
// Firebase Authentication の利用者を消す。呼んだ後はサインアウトする。
export async function deleteAccount() {
  await client.deleteAccount({});
}
