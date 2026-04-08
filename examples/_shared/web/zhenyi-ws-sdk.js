// Minimal browser SDK for zhenyi demo binary frame protocol.
(function (global) {
  "use strict";

  class ZhenyiWsClient {
    constructor(handlers) {
      this.handlers = handlers || {};
      this.ws = null;
      this.seq = 0;
      this.rxBuf = new Uint8Array(0);
      this.connected = false;
    }

    isConnected() {
      return this.connected && this.ws && this.ws.readyState === 1;
    }

    connect(url) {
      this.close();
      this.seq = 0;
      this.rxBuf = new Uint8Array(0);
      const ws = new WebSocket(url);
      ws.binaryType = "arraybuffer";
      this.ws = ws;

      ws.onopen = () => {
        this.connected = true;
        if (this.handlers.onOpen) this.handlers.onOpen();
      };
      ws.onclose = () => {
        this.connected = false;
        if (this.handlers.onClose) this.handlers.onClose();
      };
      ws.onerror = (e) => {
        if (this.handlers.onError) this.handlers.onError(e);
      };
      ws.onmessage = (ev) => {
        if (typeof ev.data === "string") {
          if (this.handlers.onText) this.handlers.onText(ev.data);
          return;
        }
        this.rxBuf = this._mergeBuf(this.rxBuf, new Uint8Array(ev.data));
        const used = this._parsePackets(this.rxBuf);
        if (used > 0) this.rxBuf = this.rxBuf.subarray(used);
      };
    }

    close() {
      if (this.ws) {
        try {
          this.ws.close();
        } catch (_) {}
      }
      this.ws = null;
      this.connected = false;
    }

    sendJson(msgId, obj) {
      if (!this.isConnected()) return false;
      this.ws.send(this._packJson(msgId, obj || {}));
      return true;
    }

    _packJson(msgId, obj) {
      const body = new TextEncoder().encode(JSON.stringify(obj));
      const buf = new ArrayBuffer(12 + body.length);
      const dv = new DataView(buf);
      dv.setInt32(0, msgId, false);
      dv.setUint32(4, (++this.seq) >>> 0, false);
      dv.setUint32(8, body.length, false);
      new Uint8Array(buf, 12).set(body);
      return buf;
    }

    _mergeBuf(a, b) {
      const out = new Uint8Array(a.length + b.length);
      out.set(a, 0);
      out.set(b, a.length);
      return out;
    }

    _parsePackets(u8) {
      let off = 0;
      while (off + 12 <= u8.length) {
        const dv = new DataView(u8.buffer, u8.byteOffset + off, 12);
        const msgId = dv.getInt32(0, false);
        const len = dv.getUint32(8, false);
        if (len > (1 << 20) || off + 12 + len > u8.length) break;
        const payload = new TextDecoder().decode(u8.subarray(off + 12, off + 12 + len));
        if (this.handlers.onPacket) this.handlers.onPacket(msgId, payload);
        off += 12 + len;
      }
      return off;
    }
  }

  global.ZhenyiWsClient = ZhenyiWsClient;
})(window);
