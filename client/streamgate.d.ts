export interface StreamgateOptions {
  token?: string;
  userId?: string;
  userInfo?: Record<string, any>;
  lastEventId?: string;
}

export interface PresenceMember {
  client_id: string;
  user_id: string;
  info: Record<string, any>;
  joined_at?: string;
}

export declare class Channel {
  readonly name: string;
  readonly isPresence: boolean;
  readonly members: Map<string, PresenceMember>;

  on<T = any>(event: string, callback: (data: T, lastEventId?: string) => void): this;
  off(event: string, callback: (...args: any[]) => void): this;
}

export declare class Streamgate {
  constructor(baseUrl?: string, options?: StreamgateOptions);

  subscribe(channelName: string): Channel;
  unsubscribe(channelName: string): void;
  connect(): this;
  disconnect(): void;

  on(type: 'open' | 'error' | 'message', callback: (...args: any[]) => void): this;
  getHistory<T = any>(channel: string, limit?: number): Promise<{ channel: string; count: number; events: T[] }>;
  getMembers(channel: string): Promise<{ channel: string; members_count: number; members: PresenceMember[] }>;
}
