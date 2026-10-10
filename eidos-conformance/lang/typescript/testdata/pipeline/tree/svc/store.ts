/**
 * Session is one signed-in user's state.
 */
export interface Session {
  id: string;
  expires: bigint;
}

/**
 * Store is the persistence seam for sessions.
 */
// +acme:stub tag=test
export interface Store {
  get(key: string): Promise<Session>;
  put(s: Session): Promise<void>;
}
