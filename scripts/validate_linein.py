#!/usr/bin/env python3
"""
validate_linein.py — Valida a hipótese do AudioQueue idle-running no line-in.

HIPÓTESE
--------
Chamar Start() ANTES dos dados de FFmpeg chegarem coloca o AudioQueue em estado
"idle-running" (hungry): callbacks disparam na velocidade da CPU, consomem
buffers de silêncio instantaneamente e causam áudio picotado ou auto-stop.

Chamar Start() APÓS o primeiro Write() bem-sucedido elimina esse estado: o queue
começa a reprodução com dados reais já enfileirados.

CENÁRIOS
--------
A (atual/quebrado): output.start() → aguarda FFmpeg → output.write()
B (correção):       aguarda FFmpeg → output.write() pré-enfileira → output.start()

MÉTRICA
-------
Número de "callbacks de silêncio" = callbacks disparados enquanto não havia dados
de FFmpeg. Cada callback = BLOCKSIZE/SR segundos de silêncio/picote audível.

  Cenário A → N callbacks silenciosos (N > 0, geralmente 5–20+)
  Cenário B → 0 callbacks silenciosos (dados já disponíveis ao iniciar)

INSTALAÇÃO
----------
    pip install sounddevice numpy

USO
---
    # listar dispositivos
    python3 validate_linein.py --list-devices

    # executar ambos os cenários com dispositivo 2
    python3 validate_linein.py --device 2 --duration 8

    # executar só o cenário B
    python3 validate_linein.py --device 2 --scenario b
"""

import argparse
import queue as Q
import subprocess
import sys
import threading
import time

import numpy as np

try:
    import sounddevice as sd
except ImportError:
    print("ERROR: sounddevice não encontrado. Execute: pip install sounddevice numpy")
    sys.exit(1)

# ---------------------------------------------------------------------------
# Constantes — mesmas do playout engine
# ---------------------------------------------------------------------------
SR = 48000
CH = 2
BLOCKSIZE = 2048       # ~42 ms por chunk, igual ao bufferFrames do Go
DTYPE = "float32"


# ---------------------------------------------------------------------------
# Captura via FFmpeg (espelho do FFmpegCapture.go)
# ---------------------------------------------------------------------------

def _ffmpeg_proc(device_id: str, duration_s: int) -> subprocess.Popen:
    cmd = [
        "ffmpeg", "-hide_banner", "-loglevel", "error",
        "-f", "avfoundation",
        "-i", f":{device_id}",
        "-f", "f32le", "-acodec", "pcm_f32le",
        "-ac", str(CH), "-ar", str(SR),
        "-t", str(duration_s),
        "pipe:1",
    ]
    return subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def _reader_thread(proc: subprocess.Popen, pcm_q: Q.Queue, stop_ev: threading.Event):
    """Lê chunks PCM de FFmpeg e enfileira. Termina com sentinel None."""
    bytes_per_chunk = BLOCKSIZE * CH * 4  # float32 = 4 bytes
    while not stop_ev.is_set():
        raw = proc.stdout.read(bytes_per_chunk)
        if not raw or len(raw) < bytes_per_chunk:
            break
        arr = np.frombuffer(raw, dtype=np.float32).reshape(BLOCKSIZE, CH).copy()
        pcm_q.put(arr)
    pcm_q.put(None)  # sentinel


# ---------------------------------------------------------------------------
# Cenário A — start() antes dos dados (comportamento atual)
# ---------------------------------------------------------------------------

def scenario_a(device_id: str, duration_s: int) -> dict:
    """
    Espelha o código atual do handler.go + manager.go:
      1. h.out.Open()
      2. h.out.Start()          ← stream inicia sem dados
      3. mgr.Start() → FFmpeg inicia
      4. m.out.Write(frames)    ← dados chegam ~200-500ms depois
    """
    pcm_q: Q.Queue = Q.Queue(maxsize=128)
    stop_ev = threading.Event()

    silent_cb = [0]
    real_cb    = [0]
    data_ready = [False]
    first_data_t = [None]
    t0 = time.perf_counter()

    # ── callback (equivale ao goBufferReady do CoreAudio) ──────────────────
    def callback(outdata: np.ndarray, frames: int, time_info, status):
        if not data_ready[0]:
            outdata.fill(0.0)          # silêncio: sem dados ainda
            silent_cb[0] += 1
            return
        try:
            chunk = pcm_q.get_nowait()
            if chunk is None:
                outdata.fill(0.0)
                silent_cb[0] += 1
            else:
                outdata[:] = chunk
                real_cb[0] += 1
        except Q.Empty:
            outdata.fill(0.0)          # underrun mid-session
            silent_cb[0] += 1

    # ── PASSO 1-2: Open + Start imediatamente (sem dados) ──────────────────
    stream = sd.OutputStream(
        samplerate=SR, channels=CH, dtype=DTYPE,
        blocksize=BLOCKSIZE, callback=callback,
    )
    stream.start()
    stream_start_t = time.perf_counter() - t0
    print(f"  [A] stream.start() em t={stream_start_t*1000:.0f}ms  ← SEM DADOS")

    # ── PASSO 3: inicia FFmpeg (igual ao mgr.Start()) ──────────────────────
    proc = _ffmpeg_proc(device_id, duration_s)
    reader = threading.Thread(target=_reader_thread, args=(proc, pcm_q, stop_ev))
    reader.start()

    # Aguarda primeiro dado de FFmpeg (polling leve)
    while first_data_t[0] is None:
        if not pcm_q.empty():
            first_data_t[0] = time.perf_counter() - t0
            data_ready[0] = True
        time.sleep(0.001)

    print(f"  [A] 1º dado FFmpeg em t={first_data_t[0]*1000:.0f}ms")
    print(f"  [A] Callbacks silenciosos até o 1º dado: {silent_cb[0]}")
    print(f"  [A] Silêncio inicial: {silent_cb[0] * BLOCKSIZE / SR * 1000:.0f}ms")

    # Aguarda fim da sessão
    time.sleep(max(0, duration_s - first_data_t[0]) + 0.5)

    stream.stop()
    stream.close()
    stop_ev.set()
    proc.terminate()
    reader.join()

    return {
        "stream_start_ms": stream_start_t * 1000,
        "first_data_ms":   first_data_t[0] * 1000,
        "silent_cb":       silent_cb[0],
        "real_cb":         real_cb[0],
        "silent_ms":       silent_cb[0] * BLOCKSIZE / SR * 1000,
    }


# ---------------------------------------------------------------------------
# Cenário B — start() APÓS o primeiro Write() (correção proposta)
# ---------------------------------------------------------------------------

def scenario_b(device_id: str, duration_s: int) -> dict:
    """
    Espelha a correção proposta:
      1. h.out.Open()           ← sem Start()
      2. mgr.Start() → FFmpeg inicia
      3. m.out.Write(frames)    ← primeiro Write() enfileira no queue parado
      4. m.out.Start()          ← queue inicia com buffer já presente
    """
    pcm_q: Q.Queue = Q.Queue(maxsize=128)
    stop_ev = threading.Event()

    silent_cb = [0]
    real_cb    = [0]
    t0 = time.perf_counter()

    # ── callback ────────────────────────────────────────────────────────────
    def callback(outdata: np.ndarray, frames: int, time_info, status):
        try:
            chunk = pcm_q.get_nowait()
            if chunk is None:
                outdata.fill(0.0)
                silent_cb[0] += 1
            else:
                outdata[:] = chunk
                real_cb[0] += 1
        except Q.Empty:
            outdata.fill(0.0)
            silent_cb[0] += 1

    # ── PASSO 1: Open — sem Start ────────────────────────────────────────
    stream = sd.OutputStream(
        samplerate=SR, channels=CH, dtype=DTYPE,
        blocksize=BLOCKSIZE, callback=callback,
    )
    # NÃO chama stream.start() aqui

    # ── PASSO 2: inicia FFmpeg ───────────────────────────────────────────
    proc = _ffmpeg_proc(device_id, duration_s)
    reader = threading.Thread(target=_reader_thread, args=(proc, pcm_q, stop_ev))
    reader.start()

    # ── PASSO 3: bloqueia até o primeiro chunk chegar ─────────────────────
    print(f"  [B] Aguardando 1º dado de FFmpeg...")
    while pcm_q.empty():
        time.sleep(0.001)

    first_data_t = time.perf_counter() - t0
    print(f"  [B] 1º dado FFmpeg em t={first_data_t*1000:.0f}ms")

    # ── PASSO 4: Start com dados já presentes ────────────────────────────
    stream.start()
    stream_start_t = time.perf_counter() - t0
    print(f"  [B] stream.start() em t={stream_start_t*1000:.0f}ms  ← COM DADOS")
    print(f"  [B] Callbacks silenciosos esperados: 0")

    # Aguarda fim da sessão
    time.sleep(max(0, duration_s - first_data_t) + 0.5)

    stream.stop()
    stream.close()
    stop_ev.set()
    proc.terminate()
    reader.join()

    return {
        "stream_start_ms": stream_start_t * 1000,
        "first_data_ms":   first_data_t * 1000,
        "silent_cb":       silent_cb[0],
        "real_cb":         real_cb[0],
        "silent_ms":       silent_cb[0] * BLOCKSIZE / SR * 1000,
    }


# ---------------------------------------------------------------------------
# Listagem de dispositivos
# ---------------------------------------------------------------------------

def list_devices():
    print("=== Saídas de áudio disponíveis (sounddevice / PortAudio) ===")
    for i, d in enumerate(sd.query_devices()):
        if d["max_output_channels"] > 0:
            tag = " ← padrão" if i == sd.default.device[1] else ""
            print(f"  [{i:2d}] {d['name']}{tag}")

    print("\n=== Entradas avfoundation disponíveis (FFmpeg) ===")
    r = subprocess.run(
        ["ffmpeg", "-hide_banner", "-f", "avfoundation",
         "-list_devices", "true", "-i", ""],
        capture_output=True, text=True,
    )
    for line in (r.stderr + r.stdout).splitlines():
        if line.strip():
            print(" ", line)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    ap = argparse.ArgumentParser(
        description="Valida hipótese AudioQueue idle-running no line-in",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    ap.add_argument("--device", "-d", default="2",
                    help="ID do dispositivo de captura avfoundation (padrão: 2)")
    ap.add_argument("--duration", "-t", type=int, default=8,
                    help="Duração em segundos por cenário (padrão: 8)")
    ap.add_argument("--scenario", "-s", choices=["a", "b", "both"], default="both",
                    help="Cenário a executar (padrão: both)")
    ap.add_argument("--list-devices", "-l", action="store_true",
                    help="Lista dispositivos e sai")
    args = ap.parse_args()

    if args.list_devices:
        list_devices()
        return

    print("=" * 65)
    print("VALIDAÇÃO: hipótese AudioQueue idle-running no line-in")
    print("=" * 65)
    print(f"Dispositivo captura : avfoundation :{args.device}")
    print(f"Duração por cenário : {args.duration}s")
    print(f"Buffer              : {BLOCKSIZE} frames @ {SR} Hz = "
          f"{BLOCKSIZE/SR*1000:.0f} ms/chunk")
    print()

    results = {}

    if args.scenario in ("a", "both"):
        input("── Pressione Enter para CENÁRIO A (start antes dos dados) ──")
        print()
        results["a"] = scenario_a(args.device, args.duration)
        print()

    if args.scenario in ("b", "both"):
        input("── Pressione Enter para CENÁRIO B (start após primeiro chunk) ──")
        print()
        results["b"] = scenario_b(args.device, args.duration)
        print()

    # ── Relatório ──────────────────────────────────────────────────────────
    print("=" * 65)
    print("RESULTADO")
    print("=" * 65)

    if "a" in results:
        r = results["a"]
        print(f"Cenário A | FFmpeg latência: {r['first_data_ms']:.0f}ms | "
              f"silêncio inicial: {r['silent_ms']:.0f}ms "
              f"({r['silent_cb']} callbacks)")

    if "b" in results:
        r = results["b"]
        print(f"Cenário B | FFmpeg latência: {r['first_data_ms']:.0f}ms | "
              f"silêncio inicial: {r['silent_ms']:.0f}ms "
              f"({r['silent_cb']} callbacks)")

    print()

    if "a" in results and "b" in results:
        diff = results["a"]["silent_ms"] - results["b"]["silent_ms"]
        if results["b"]["silent_cb"] == 0 and results["a"]["silent_cb"] > 0:
            print("✓ HIPÓTESE CONFIRMADA")
            print(f"  Cenário B eliminou {diff:.0f}ms de silêncio inicial.")
            print("  Iniciar o queue APÓS o primeiro Write() resolve o problema.")
        elif results["b"]["silent_cb"] < results["a"]["silent_cb"]:
            print("~ HIPÓTESE PARCIALMENTE CONFIRMADA")
            print(f"  Cenário B reduziu {diff:.0f}ms de silêncio, mas ainda há "
                  f"{results['b']['silent_ms']:.0f}ms de silêncio residual.")
            print("  Investigar: possível underrun mid-session.")
        else:
            print("✗ HIPÓTESE NÃO CONFIRMADA")
            print("  Os dois cenários produziram resultados semelhantes.")
            print("  O problema pode estar em outro ponto do pipeline.")

    print()


if __name__ == "__main__":
    main()
