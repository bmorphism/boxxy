---
name: cognionics-hd72
description: "Cognionics HD-72 64CH EEG acquisition via FTDI serial dongle. Protocol FULLY REVERSED and LIVE VALIDATED. Legacy 0xFF-sync format. 3 Mbaud. 67 channels (64 EEG + 3 ACC). 207 bytes/packet at ~333 Hz. 100% counter continuity confirmed."
license: MIT
compatibility: macOS (FTDI VCP driver), Linux. Requires pyserial or direct POSIX termios. FTDI FT232R dongle (VID=0x0403 PID=0x6001).
metadata:
  version: 0.2.0
  device: "Cognionics HD-72 64CH 1702HDG"
  dongle-chip: "FTDI FT232R"
  baud: 3000000
  channels: 67
  channel-breakdown: "64 EEG + 3 ACC (counter/trigger/battery in packet overhead)"
  sample-rate: "~333 Hz"
  packet-size: 207
  adc: "24-bit (ADS1299-family)"
  wireless: "Proprietary 2.4 GHz (Nordic nRF-family)"
  encoding: "legacy 0xFF-sync, non-standard 3-byte channel packing"
  gf3-trit: "-1"
  trit-role: "SENSOR (raw acquisition)"
  protocol-status: "fully-reversed-and-validated"
allowed-tools: "Bash(python3:*) Bash(cc:*) Bash(zig:*) Read"
---

# Cognionics HD-72 64CH EEG Streaming

## Hardware

- **Headset**: Cognionics HD-72 64CH, model 1702HDG
- **Dongle**: FTDI FT232R USB-UART (VID=0x0403, PID=0x6001, serial AI1B2OSR)
- **Port**: `/dev/cu.usbserial-AI1B2OSR` (macOS)
- **Baud**: **3,000,000 (3 Mbaud), 8N1** — NOT 1.5 Mbaud!
- **Throughput**: ~69 KB/s streaming, 1-2 KB/s idle (wireless-gated)
- **Sample Rate**: ~333 Hz

## Protocol (FULLY REVERSED & VALIDATED)

### Packet Structure (207 bytes)

```
[0xFF sync] [counter mod 128] [67 × 3B channels] [tail] [battery] [trigger_hi] [trigger_lo]
```

| Field    | Bytes | Description |
|----------|-------|-------------|
| Sync     | 1     | `0xFF` |
| Counter  | 1     | Mod 128, increments by 1 per sample |
| Channels | 201   | 67 channels × 3 bytes (64 EEG + 3 ACC) |
| Tail     | 1     | `0x11` = impedance ON, `0x10` = impedance OFF |
| Battery  | 1     | Raw byte, scale by BatteryGain |
| Trigger  | 2     | Big-endian 16-bit |

### Channel Bit Packing (NON-STANDARD)

Each 3-byte channel uses Cognionics non-standard packing:

```c
raw = (msb << 24) | (lsb2 << 17) | (lsb1 << 10);
value = raw >> 8;  // arithmetic shift (sign-extends)
uV = value * (1e6 / 4294967296.0);
```

This is NOT standard big-endian `(msb << 16) | (mid << 8) | lsb`.

### Channel Layout

| Index  | Type         | Count | Notes |
|--------|--------------|-------|-------|
| 0..63  | EEG          | 64    | Remap via `hd72.map` for electrode positions |
| 64..66 | Accelerometer| 3     | Post-decode shift `<<5` for full range |

### Critical: Baud Rate

**MUST be 3 Mbaud, not 1.5 Mbaud.** At 1.5 Mbaud the FTDI chip negotiates a connection
but introduces bit errors that destroy the 0xFF sync bytes, making the stream appear
unframed with high entropy. This caused weeks of misdiagnosis.

The CGX Acquisition software uses FTDI D2XX direct API at 3 Mbaud. The macOS VCP driver
also supports 3 Mbaud via pyserial `serial.Serial(port, 3000000)`.

## Live Decoder (Python)

```python
import serial, time

NUM_CH = 67
PKT_SIZE = 207
UV_SCALE = 1e6 / (2**32)

def decode_ch(msb, lsb2, lsb1):
    raw = (msb << 24) | (lsb2 << 17) | (lsb1 << 10)
    if raw >= 2**31: raw -= 2**32
    return raw >> 8

ser = serial.Serial('/dev/cu.usbserial-AI1B2OSR', 3000000, timeout=1)
time.sleep(0.3)
ser.reset_input_buffer()
ser.write(b'\x12')  # strip impedance

buf = bytearray()
t0 = time.time()
while time.time() - t0 < 5:
    d = ser.read(8192)
    if d: buf.extend(d)
ser.close()

# Find sync and decode
pos = buf.index(0xFF)
while pos + PKT_SIZE <= len(buf):
    if buf[pos] != 0xFF:
        pos += 1; continue
    counter = buf[pos+1]
    channels = []
    for ch in range(NUM_CH):
        base = pos + 2 + ch*3
        channels.append(decode_ch(buf[base], buf[base+1], buf[base+2]))
    pos += PKT_SIZE
    eeg_uv = [c * UV_SCALE for c in channels[:64]]
    print(f'#{counter:3d}: ch0={eeg_uv[0]:.1f}uV')
```

Full decoder with sync recovery, counter tracking, and channel stats:
`~/i/cgx_legacy_decode.py`

## Impedance

- GAIN=3.0, VREF=2.5V, ISTIM=24nA
- Tail byte `0x11` enables impedance interleave
- `ADC_TO_VOLTS = 2 * (2.5 / (2^32 * 3.0))`

## Related Trees

- `bcf-0052` — HD-72 pipeline design
- `bcf-0053` — Live serial acquisition log (this skill's empirical source)
- `cgt-0002` — Resource sharing game, 64ch montage

## Revision History

- v0.1.0: Initial delta-compressed protocol hypothesis (INCORRECT)
- v0.2.0: **CORRECTED** to legacy 0xFF-sync format. Live validated at 3 Mbaud.
  Root cause of earlier confusion: 1.5 Mbaud bit errors destroyed sync bytes.
