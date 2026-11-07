/**
 * streamgate client library (zero dependencies, < 2KB minified)
 * Browser and Node.js Server-Sent Events client for streamgate
 */

class Channel {
  constructor(name, client) {
    this.name = name;
    this.client = client;
    this.listeners = new Map();
    this.members = new Map();
    this.isPresence = name.startsWith('presence-');

    if (this.isPresence) {
      this.on('presence.member_added', (data) => {
        if (data && data.user_id) {
          this.members.set(data.user_id, data);
        }
      });
      this.on('presence.member_removed', (data) => {
        if (data && data.user_id) {
          this.members.delete(data.user_id);
        }
      });
    }
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event).add(callback);
    return this;
  }

  off(event, callback) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).delete(callback);
    }
    return this;
  }

  emit(event, data, lastEventId) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach((cb) => {
        try {
          cb(data, lastEventId);
        } catch (err) {
          console.error(`[streamgate] Error in event listener for ${event}:`, err);
        }
      });
    }
  }
}

class Streamgate {
  constructor(baseUrl, options = {}) {
    this.baseUrl = (baseUrl || '').replace(/\/$/, '');
    this.token = options.token || null;
    this.userId = options.userId || null;
    this.userInfo = options.userInfo || null;
    this.channels = new Map();
    this.eventSource = null;
    this.lastEventId = options.lastEventId || null;
    this.isConnected = false;
    this.listeners = { open: new Set(), error: new Set(), message: new Set() };
  }

  subscribe(channelName) {
    if (!this.channels.has(channelName)) {
      const ch = new Channel(channelName, this);
      this.channels.set(channelName, ch);

      // If already connected, reconnect to include new channel
      if (this.isConnected) {
        this.connect();
      }
    }
    return this.channels.get(channelName);
  }

  unsubscribe(channelName) {
    this.channels.delete(channelName);
    if (this.isConnected) {
      this.connect();
    }
  }

  connect() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    const channelList = Array.from(this.channels.keys());
    if (channelList.length === 0) {
      return this;
    }

    const params = new URLSearchParams();
    params.set('channel', channelList.join(','));
    if (this.token) params.set('token', this.token);
    if (this.userId) params.set('user_id', this.userId);
    if (this.userInfo) params.set('info', JSON.stringify(this.userInfo));
    if (this.lastEventId) params.set('last_event_id', this.lastEventId);

    const streamUrl = `${this.baseUrl}/v1/subscribe?${params.toString()}`;
    const es = new EventSource(streamUrl);
    this.eventSource = es;

    es.onopen = () => {
      this.isConnected = true;
      this.listeners.open.forEach((cb) => cb());
    };

    es.onerror = (err) => {
      this.isConnected = false;
      this.listeners.error.forEach((cb) => cb(err));
    };

    es.onmessage = (e) => {
      if (e.lastEventId) this.lastEventId = e.lastEventId;
      this.dispatchToChannels('message', e.data, e.lastEventId);
    };

    // Forward custom events
    const allRegisteredEvents = new Set();
    this.channels.forEach((ch) => {
      ch.listeners.forEach((_, eventName) => allRegisteredEvents.add(eventName));
    });

    allRegisteredEvents.forEach((eventName) => {
      es.addEventListener(eventName, (e) => {
        if (e.lastEventId) this.lastEventId = e.lastEventId;
        let parsed = e.data;
        try {
          parsed = JSON.parse(e.data);
        } catch {
          // keep as string
        }
        this.dispatchToChannels(eventName, parsed, e.lastEventId);
      });
    });

    return this;
  }

  disconnect() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
    this.isConnected = false;
  }

  dispatchToChannels(event, data, lastEventId) {
    this.listeners.message.forEach((cb) => cb(event, data, lastEventId));
    this.channels.forEach((ch) => ch.emit(event, data, lastEventId));
  }

  on(type, callback) {
    if (this.listeners[type]) {
      this.listeners[type].add(callback);
    }
    return this;
  }

  async getHistory(channel, limit = 50) {
    const res = await fetch(`${this.baseUrl}/api/v1/channels/${encodeURIComponent(channel)}/history?limit=${limit}`);
    if (!res.ok) throw new Error(`History fetch failed: HTTP ${res.status}`);
    return res.json();
  }

  async getMembers(channel) {
    const res = await fetch(`${this.baseUrl}/api/v1/channels/${encodeURIComponent(channel)}/members`);
    if (!res.ok) throw new Error(`Members fetch failed: HTTP ${res.status}`);
    return res.json();
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { Streamgate, Channel };
} else if (typeof window !== 'undefined') {
  window.Streamgate = Streamgate;
}
