# Noble Genesis VirtioFS Share

This directory is shared with the boxxy VM guest via VirtioFS.
The guest mounts it as: `mount -t virtiofs noble-genesis /mnt/noble-genesis`

## Required Files

1. **genesis.json** — Noble mainnet genesis
   ```bash
   curl -sL https://raw.githubusercontent.com/strangelove-ventures/noble-networks/main/mainnet/noble-1/genesis.json > genesis.json
   ```

2. **nobled** — Noble daemon binary (linux/arm64)
   ```bash
   git clone --depth 1 --branch v11.2.0 https://github.com/strangelove-ventures/noble.git
   cd noble && GOOS=linux GOARCH=arm64 make build
   cp build/nobled ../noble-genesis/nobled
   ```

3. **provision-noble-guest.sh** — Auto-provisioning script (copied from scripts/)
   ```bash
   cp ../../scripts/provision-noble-guest.sh .
   ```

4. **snapshot.tar.lz4** (optional) — Polkachu pruned snapshot (~1 GB)
   ```bash
   wget -O snapshot.tar.lz4 https://snapshots.polkachu.com/snapshots/noble/noble_46419300.tar.lz4
   ```

## Usage

```bash
# From boxxy host:
boxxy examples/noble-vm.joke

# Inside guest after boot:
mount -t virtiofs noble-genesis /mnt/noble-genesis
sudo /mnt/noble-genesis/provision-noble-guest.sh
```
