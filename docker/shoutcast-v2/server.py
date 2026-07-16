#!/usr/bin/env python3
"""
SHOUTcast v2 / Icecast test relay server.

  Port 8002 — source input  : FFmpeg / playout engine conectam aqui
  Port 9001 — listener output: VLC / browser conectam aqui para ouvir

Protocolo: HTTP SOURCE com Authorization Basic base64("source:{password}")
FFmpeg usa icecast://source:{password}@host:8002/stream (sem -legacy_icecast)
"""

import base64
import queue
import socket
import threading
import time

HOST          = "0.0.0.0"
SOURCE_PORT   = 8002
LISTENER_PORT = 9001
PASSWORD      = "radiotest"
CONTENT_TYPE  = "audio/mpeg"   # ajuste para audio/ogg se usar ogg_vorbis

# ── listener registry ─────────────────────────────────────────────────────────
_listeners: list[queue.Queue] = []
_listeners_lock = threading.Lock()

def _add_listener(q: queue.Queue):
    with _listeners_lock:
        _listeners.append(q)

def _remove_listener(q: queue.Queue):
    with _listeners_lock:
        try:
            _listeners.remove(q)
        except ValueError:
            pass

def _broadcast(chunk: bytes):
    with _listeners_lock:
        for q in list(_listeners):
            try:
                q.put_nowait(chunk)
            except queue.Full:
                pass

# ── source handler ────────────────────────────────────────────────────────────
def handle_source(conn: socket.socket, addr):
    print(f"[src] Connected from {addr[0]}:{addr[1]}", flush=True)
    total = 0
    start = time.monotonic()

    try:
        # ── Read headers ──────────────────────────────────────────────────────
        raw = b""
        while b"\r\n\r\n" not in raw:
            chunk = conn.recv(4096)
            if not chunk:
                print("[src] Disconnected before headers", flush=True)
                return
            raw += chunk

        sep          = raw.index(b"\r\n\r\n")
        header_bytes = raw[:sep]
        body_start   = raw[sep + 4:]
        header       = header_bytes.decode("latin-1", errors="replace")

        print(f"[src] Headers:\n{header}\n", flush=True)

        # ── Parse request line ────────────────────────────────────────────────
        # SHOUTcast v2 / Icecast: "SOURCE /mount HTTP/1.0" or "PUT /mount HTTP/1.1"
        lines      = header.split("\r\n")
        first_line = lines[0]
        method     = first_line.split(" ")[0] if first_line else ""
        mount      = first_line.split(" ")[1] if len(first_line.split(" ")) > 1 else "/"

        print(f"[src] Method={method}  Mount={mount}", flush=True)

        # ── Validate password ─────────────────────────────────────────────────
        authed = False
        for line in lines[1:]:
            low = line.lower()
            if low.startswith("authorization:"):
                value = line.split(":", 1)[1].strip()
                if value.lower().startswith("basic "):
                    try:
                        decoded = base64.b64decode(value[6:]).decode()
                        # SHOUTcast v2 / Icecast: "source:{password}"
                        if ":" in decoded and decoded.split(":", 1)[1] == PASSWORD:
                            authed = True
                    except Exception:
                        pass

        if authed:
            print(f"[src] Auth OK", flush=True)
        else:
            print(f"[src] Warning: password not matched — accepting anyway for diagnostics", flush=True)

        # ── Respond ───────────────────────────────────────────────────────────
        # Icecast / SHOUTcast v2 expects HTTP 200
        if method == "PUT":
            conn.sendall(b"HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 200 OK\r\n\r\n")
        else:
            # SOURCE method — Icecast responds with ICY 200 OK
            conn.sendall(b"ICY 200 OK\r\n\r\n")

        print("[src] Stream accepted — relaying to listeners...", flush=True)

        if body_start:
            _broadcast(body_start)
            total += len(body_start)

        # ── Drain and broadcast ───────────────────────────────────────────────
        while True:
            chunk = conn.recv(65536)
            if not chunk:
                break
            _broadcast(chunk)
            total += len(chunk)
            elapsed = time.monotonic() - start
            if elapsed > 0 and total % (128 * 1024) < 65536:
                kbps = (total * 8) / elapsed / 1000
                print(f"[src] {total // 1024} KB | {kbps:.0f} kbps avg", flush=True)

    except ConnectionResetError:
        print("[src] Source disconnected (reset)", flush=True)
    except Exception as e:
        print(f"[src] Error: {e}", flush=True)
    finally:
        elapsed = time.monotonic() - start
        print(f"[src] Session ended — {total} bytes in {elapsed:.1f}s", flush=True)
        conn.close()

# ── listener handler ──────────────────────────────────────────────────────────
def handle_listener(conn: socket.socket, addr):
    print(f"[lis] Listener connected from {addr[0]}:{addr[1]}", flush=True)
    q: queue.Queue = queue.Queue(maxsize=128)
    _add_listener(q)

    try:
        raw = b""
        conn.settimeout(3.0)
        try:
            while b"\r\n\r\n" not in raw:
                chunk = conn.recv(4096)
                if not chunk:
                    return
                raw += chunk
        except TimeoutError:
            pass
        conn.settimeout(None)

        response = (
            f"HTTP/1.0 200 OK\r\n"
            f"Content-Type: {CONTENT_TYPE}\r\n"
            f"icy-name: RadioFlow SHOUTcast v2 Test\r\n"
            f"icy-genre: Test\r\n"
            f"icy-br: 128\r\n"
            f"\r\n"
        )
        conn.sendall(response.encode())

        while True:
            try:
                chunk = q.get(timeout=10)
                conn.sendall(chunk)
            except queue.Empty:
                pass
            except (BrokenPipeError, ConnectionResetError):
                break

    except Exception as e:
        print(f"[lis] Error: {e}", flush=True)
    finally:
        _remove_listener(q)
        conn.close()
        print(f"[lis] Listener {addr[0]}:{addr[1]} disconnected", flush=True)

# ── main ──────────────────────────────────────────────────────────────────────
def serve(port: int, handler):
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((HOST, port))
    srv.listen(20)
    while True:
        conn, addr = srv.accept()
        threading.Thread(target=handler, args=(conn, addr), daemon=True).start()

def main():
    print(f"[*] SHOUTcast v2 / Icecast test relay", flush=True)
    print(f"[*] Source  port : {SOURCE_PORT}   (FFmpeg conecta aqui)", flush=True)
    print(f"[*] Listener port: {LISTENER_PORT}  (VLC / browser ouve aqui)", flush=True)
    print(f"[*] Password     : {PASSWORD}\n", flush=True)

    threading.Thread(target=serve, args=(LISTENER_PORT, handle_listener), daemon=True).start()
    serve(SOURCE_PORT, handle_source)

if __name__ == "__main__":
    main()
