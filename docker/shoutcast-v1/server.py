#!/usr/bin/env python3
"""
SHOUTcast v1 test relay server.

  Port 8001 — source input  : FFmpeg / playout engine conectam aqui
  Port 9000 — listener output: VLC / browser conectam aqui para ouvir

O áudio recebido do source é retransmitido em tempo real para todos
os ouvintes conectados na porta 9000.
"""

import queue
import socket
import threading
import time

HOST          = "0.0.0.0"
SOURCE_PORT   = 8001
LISTENER_PORT = 9000
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
                pass  # ouvinte lento — descarta frame

# ── source handler ────────────────────────────────────────────────────────────
def handle_source(conn: socket.socket, addr):
    print(f"[src] Connected from {addr[0]}:{addr[1]}", flush=True)
    total = 0
    start = time.monotonic()

    try:
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

        print(f"[src] Headers:\n{header_bytes.decode('latin-1', errors='replace')}\n", flush=True)

        conn.sendall(b"ICY 200 OK\r\n\r\n")
        print("[src] Sent ICY 200 OK — streaming to listeners...", flush=True)

        if body_start:
            _broadcast(body_start)
            total += len(body_start)

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
        # Read HTTP GET request (just drain it)
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

        # Send HTTP/1.0 audio stream response
        response = (
            f"HTTP/1.0 200 OK\r\n"
            f"Content-Type: {CONTENT_TYPE}\r\n"
            f"icy-name: RadioFlow Local Test\r\n"
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
    print(f"[*] Source  port : {SOURCE_PORT}   (FFmpeg conecta aqui)", flush=True)
    print(f"[*] Listener port: {LISTENER_PORT}  (VLC / browser ouve aqui)", flush=True)
    print(f"[*] Password     : {PASSWORD}\n", flush=True)

    threading.Thread(target=serve, args=(LISTENER_PORT, handle_listener), daemon=True).start()
    serve(SOURCE_PORT, handle_source)

if __name__ == "__main__":
    main()
