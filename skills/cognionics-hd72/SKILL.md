---
name: cognionics-hd72
description: Cognionics HD-72 64CH EEG acquisition via FTDI serial dongle. Protocol reverse-engineered from serial captures, App-Cognionics source, and OpenViBE driver. The HD-72 1702HDG firmware uses a proprietary unframed protocol incompatible with the documented 0xFF-sync format.
license: MIT
compatibility: macOS (FTDI VCP driver), Linux. Requires pyserial or direct POSIX termios. FTDI FT232R dongle (VID=0x0403 PID=0x6001).
metadata:
  version: 0.1.0
  device: "Cognionics HD-72 64CH 1702HDG"
  dongle-chip: "FTDI FT232R"
  baud: 1500000
  channels: 64
  adc: "24-bit (ADS1299-family)"
  wireless: "Proprietary 2.4 GHz (Nordic nRF-family)"
  gf3-trit: "-1"
  trit-role: "SENSOR (raw acquisition)"
  protocol-status: "partially-reversed"
allowed-tools: "Bash(python3:*) Bash(cc:*) Bash(zig:*) Read"
---

# Cognionics HD-72 64CH EEG Streaming

## Hardware

- **Headset**: Cognionics HD-72 64CH, model 1702HDG
- **Dongle**: FTDI FT232R USB-UART (VID=0x0403, PID=0x6001, serial AI1B2OSR)
- **Port**: `/dev/cu.usbserial-AI1B2OSR` (macOS)
- **Baud**: 1,500,000 (1.5 Mbaud), 8N1
- **Throughput**: 55-77 KB/s streaming, 1-2 KB/s idle (wireless-gated)

## Protocol Status

### What We Know

1. **Always streaming** — no start/stop command required, device ignores unknown bytes
2. **0x12 command** sustains full-rate streaming (55-77 KB/s). Single send, no keepalive needed.
3. **0x11 command** starts streaming with impedance interleave (52 KB/s). Keepalive floods corrupt data.
4. **Unframed** — no 0xFF sync byte, no counter, no header/tail at any candidate packet size
5. **High entropy** (6.83 bits/byte) — consistent with raw 24-bit ADC at mid-rail DC bias
6. **3-byte channel grouping** confirmed via XOR differential analysis (period-3 structure)
7. **104-byte autocorrelation** peak (corr=0.16) with harmonics at 208, 311 — ADC signal correlation, not packet framing
8. **Best packet candidate**: 69 channels × 3 bytes = 207 bytes/sample at ~314 Hz
9. **NOT encrypted** — XOR analysis proves ADC structure preserved, no cipher key period detected
10. **FT232R is transparent** — no data transformation by dongle chip
11. **Radio whitening** (nRF PN9) stripped at receiver before UART — invisible to host

### What We Don't Know

- Exact channel count per sample (64? 69? other?)
- Byte ordering within 3-byte samples (standard big-endian or Cognionics non-standard shifts?)
- Whether bit packing matches old protocol: `(msb<<24)|(lsb2<<17)|(lsb1<<10)` or standard `(msb<<16)|(mid<<8)|lsb`
- Sample boundaries (no sync/framing to anchor alignment)
- Presence of trigger/impedance/counter bytes interleaved with channel data

### Old Protocol (NOT used by HD-72 1702HDG)

The documented Cognionics protocol (from App-Cognionics and OpenViBE) uses:
```
[0xFF sync] [counter mod 128] [N × 3B channels] [0x10 or 0x11 tail]
```

Bit packing (NON-STANDARD):
```c
raw = (msb << 24) | (lsb2 << 17) | (lsb1 << 10);
uV  = raw * (1e6 / 4294967296.0);
```

The HD-72 1702HDG emits NONE of this structure. Zero 0xFF bytes in 122K samples. Completely flat per-position variance.

## Acquisition Scripts

### Quick Capture (Python)

```python
import serial, time

ser = serial.Serial('/dev/cu.usbserial-AI1B2OSR', 1500000, timeout=1)
time.sleep(0.3)
ser.reset_input_buffer()
ser.write(b'\x12')  # start streaming (strip impedance)

buf = bytearray()
t0 = time.time()
while time.time() - t0 < 5:
    d = ser.read(8192)
    if d:
        buf.extend(d)

ser.close()
with open('/tmp/cognionics-raw.bin', 'wb') as f:
    f.write(buf)
print(f'{len(buf)} bytes captured ({len(buf)/(time.time()-t0):.0f} B/s)')
```

### C Streamer (macOS, 1.5Mbaud via IOSSIOSPEED)

Pre-built at `~/i/cognionics-fast.c`. Compile:
```bash
cc -O2 -o cognionics-fast cognionics-fast.c -framework IOKit -framework CoreFoundation -lpthread
```

### Existing Parsers (old protocol, need adaptation)

- **Zig**: `~/worlds/color/ergodic/zig-syrup-cognionics/src/cognionics_parser.zig` (18K, propagator cell)
- **Clojure**: `~/worlds/color/plus/brainfloj-cognionics/src/brainfloj/cognionics/serial.clj` (jssc)
- **C**: `~/i/cognionics-fast.c` (POSIX termios + IOSSIOSPEED)
- **Python**: `~/i/cognionics-stream.py` (pyserial + LSL)

## Paths to Full Streaming

1. **CGX Acquisition** (Windows VM) — proprietary app outputs LSL. USB passthrough FTDI dongle.
2. **Contact CGX** — info@cognionics.com, they provide protocol details for custom applications.
3. **NeuroPype Academic Edition** — free, inspect CognionicsInput Python source for HD-72 code path.
4. **Known-plaintext sniff** — run CGX software + raw serial capture simultaneously, correlate to reverse-engineer framing.

## Related Trees

- `bcf-0052` — HD-72 pipeline design
- `bcf-0053` — Live serial acquisition log (this skill's empirical source)
- `cgt-0002` — Resource sharing game, 64ch montage

## ADC Physics

The ADS1299 biases inputs to Vref/2 (~2.4V on 4.8V ref). 24-bit samples at mid-rail produce near-uniform byte distribution (explaining 6.83 bits/byte entropy). MSB of each 3-byte group clusters around 0x7F-0x80. This is raw physics, not scrambling.

## Impedance

- GAIN=3.0, VREF=2.5V, ISTIM=24nA
- Impedance Z = 1.4 / (ISTIM × 2.0) × |alternating sample difference| × ADC_TO_VOLTS
- 4-sample ring buffer per channel for impedance computation
