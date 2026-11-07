export interface GatewayStats {
  active_connections: number;
  total_connections_count: number;
  total_channels_count: number;
  total_messages_published: number;
  total_messages_delivered: number;
  total_messages_dropped: number;
  uptime_seconds: number;
}

export interface RuntimeStats {
  goroutines: number;
  alloc_bytes: number;
  total_alloc_bytes: number;
  sys_bytes: number;
  num_gc: number;
  heap_objects: number;
  alloc_mb: string;
  timestamp: number;
}

export interface ChannelInfo {
  name: string;
  subscribers: number;
  total_published: number;
  total_delivered: number;
  buffer_size: number;
  buffer_capacity: number;
  created_at: string;
  last_published: string;
  is_presence?: boolean;
  member_count?: number;
}

export interface LiveEventItem {
  id: string;
  channel: string;
  event: string;
  data: string;
  timestamp: string;
  isReplay?: boolean;
}

export interface PresenceMember {
  client_id: string;
  user_id: string;
  info: Record<string, any>;
  joined_at?: string;
}
