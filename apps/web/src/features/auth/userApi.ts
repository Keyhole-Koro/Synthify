import { createRPCClient } from '@/lib/connect';
import { UserService } from '@/gen/proto/synthify/app/v1/user_pb';

const client = createRPCClient(UserService);

/**
 * Calls backend UserService.SignInUser to idempotently upsert user/account records
 * and grant initial signup credits on first login.
 *
 * @returns Promise resolving to the SignInUserResponse.
 */
export async function signInUser() {
  return await client.signInUser({});
}
