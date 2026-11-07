import React, { useState } from 'react';
import { KeyRound, Copy, Check, ShieldCheck, X } from 'lucide-react';

interface Props {
  onClose: () => void;
}

export const JwtGenerator: React.FC<Props> = ({ onClose }) => {
  const [secret, setSecret] = useState('super-secret-key-change-me');
  const [userId, setUserId] = useState('usr_dev_101');
  const [channels, setChannels] = useState('chat.*,finance.btc,presence-*');
  const [expiresHours, setExpiresHours] = useState('24');
  const [token, setToken] = useState('');
  const [copied, setCopied] = useState(false);
  const [generating, setGenerating] = useState(false);

  const base64UrlEncode = (str: string): string => {
    return btoa(str)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
  };

  const bufferToBase64Url = (buf: ArrayBuffer): string => {
    const bytes = new Uint8Array(buf);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
  };

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault();
    setGenerating(true);

    try {
      const header = { alg: 'HS256', typ: 'JWT' };
      const now = Math.floor(Date.now() / 1000);
      const exp = now + parseInt(expiresHours || '24', 10) * 3600;

      const channelList = channels
        .split(',')
        .map(c => c.trim())
        .filter(Boolean);

      const payload = {
        sub: userId,
        iat: now,
        exp,
        channels: channelList,
      };

      const encodedHeader = base64UrlEncode(JSON.stringify(header));
      const encodedPayload = base64UrlEncode(JSON.stringify(payload));
      const dataToSign = `${encodedHeader}.${encodedPayload}`;

      // Web Crypto HMAC-SHA256
      const enc = new TextEncoder();
      const cryptoKey = await crypto.subtle.importKey(
        'raw',
        enc.encode(secret),
        { name: 'HMAC', hash: 'SHA-256' },
        false,
        ['sign']
      );

      const signature = await crypto.subtle.sign('HMAC', cryptoKey, enc.encode(dataToSign));
      const encodedSignature = bufferToBase64Url(signature);

      setToken(`${dataToSign}.${encodedSignature}`);
    } catch (err) {
      console.error('Failed to generate token:', err);
    } finally {
      setGenerating(false);
    }
  };

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full p-6 shadow-2xl relative">
        <button
          onClick={onClose}
          className="absolute right-4 top-4 text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="flex items-center gap-2.5 mb-1">
          <div className="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
            <KeyRound className="w-5 h-5" />
          </div>
          <h3 className="text-base font-semibold text-white">Subscriber JWT Token Generator</h3>
        </div>
        <p className="text-xs text-slate-400 mb-5">
          Generate HMAC-SHA256 signed subscriber tokens with channel permission claims.
        </p>

        <form onSubmit={handleGenerate} className="space-y-4 text-xs font-mono">
          <div>
            <label className="block text-slate-300 font-medium mb-1">JWT Secret</label>
            <input
              type="text"
              required
              value={secret}
              onChange={e => setSecret(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-emerald-500"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-300 font-medium mb-1">User ID (sub)</label>
              <input
                type="text"
                required
                value={userId}
                onChange={e => setUserId(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-emerald-500"
              />
            </div>
            <div>
              <label className="block text-slate-300 font-medium mb-1">Expires (Hours)</label>
              <input
                type="number"
                min="1"
                max="8760"
                value={expiresHours}
                onChange={e => setExpiresHours(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-emerald-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-slate-300 font-medium mb-1">
              Authorized Channels (comma-separated, supports <code className="text-emerald-400">*</code>)
            </label>
            <input
              type="text"
              required
              value={channels}
              onChange={e => setChannels(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-emerald-500"
            />
          </div>

          <button
            type="submit"
            disabled={generating}
            className="w-full py-2.5 bg-emerald-500 hover:bg-emerald-600 text-slate-950 font-bold rounded-lg transition flex items-center justify-center gap-2"
          >
            <ShieldCheck className="w-4 h-4" />
            <span>{generating ? 'Signing...' : 'Sign JWT Token'}</span>
          </button>
        </form>

        {token && (
          <div className="mt-5 pt-4 border-t border-slate-800 space-y-3">
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-[11px] uppercase tracking-wider text-slate-400 font-mono">
                  Signed JWT Token
                </span>
                <button
                  onClick={() => handleCopy(token)}
                  className="text-emerald-400 hover:text-emerald-300 flex items-center gap-1 text-[11px] font-mono"
                >
                  {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copied ? 'Copied' : 'Copy'}</span>
                </button>
              </div>
              <div className="bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-[11px] font-mono text-emerald-300 break-all max-h-24 overflow-y-auto">
                {token}
              </div>
            </div>

            <div>
              <span className="text-[11px] uppercase tracking-wider text-slate-400 font-mono block mb-1">
                Example Subscription URL
              </span>
              <div className="bg-slate-950 border border-slate-800 rounded-lg p-2 text-[10px] font-mono text-slate-300 break-all select-all">
                {`/v1/subscribe?channel=${encodeURIComponent(channels.split(',')[0])}&token=${token}`}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
