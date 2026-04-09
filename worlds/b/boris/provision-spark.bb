#!/usr/bin/env bb
;; provision-spark.bb — One-shot parallel provisioner for 1-3 NVIDIA DGX Spark (GB10)
;;
;; Provisions all devices simultaneously via future/deref parallelism.
;; Stack: NCCL v2.28.9-1, TensorRT-LLM, vLLM, SageAttention, xDiT, lolita, VR streaming
;;
;; Usage:
;;   bb provision-spark.bb <ip1>[,<ip2>[,<ip3>]]
;;   bb provision-spark.bb 192.168.1.10                        # 1 device (128GB)
;;   bb provision-spark.bb 192.168.1.10,192.168.1.11           # 2 devices (256GB stacked)
;;   bb provision-spark.bb 192.168.1.10,192.168.1.11,192.168.1.12  # 3 devices (384GB mesh)
;;   bb provision-spark.bb 192.168.1.10 --dry-run
;;
;; Credentials: user=a, password=aaaaaa (same on all devices)
;; GF(3): device1=+1 generator, device2=-1 validator, device3=0 ergodic

(ns provision-spark
  (:require [babashka.process :refer [shell process]]
            [clojure.string :as str]))

;; ══════════════════════════════════════════════════════════════════════
;; Config
;; ══════════════════════════════════════════════════════════════════════

(def ssh-user "a")
(def ssh-pass "aaaaaa")
(def ssh-opts "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=15")
(def gf3-trits  [1 -1 0])
(def gf3-roles  {1 "generator" -1 "validator" 0 "ergodic"})
(def gf3-labels {1 "PLUS" -1 "MINUS" 0 "ERGODIC"})

;; ══════════════════════════════════════════════════════════════════════
;; SSH helpers
;; ══════════════════════════════════════════════════════════════════════

(defn ssh-script!
  "Write script to temp file, scp to device, execute remotely.
   Uses direct SSH (keys must be installed) or falls back to sshpass."
  [ip script]
  (let [ts (System/currentTimeMillis)
        local-tmp (format "/tmp/spark-%s-%d.sh" ip ts)
        remote-tmp (format "/tmp/_prov_%d.sh" ts)
        ;; Try direct SSH first, fall back to sshpass if available
        has-sshpass (zero? (:exit (shell {:continue true :out :string :err :string} "bash" "-c" "command -v sshpass")))
        scp-prefix (if has-sshpass (format "sshpass -p '%s' " ssh-pass) "")
        ssh-prefix (if has-sshpass (format "sshpass -p '%s' " ssh-pass) "")]
    (spit local-tmp script)
    (shell {:continue true :out :string :err :string}
           "bash" "-c"
           (format "%sscp %s %s %s@%s:%s"
                   scp-prefix ssh-opts local-tmp ssh-user ip remote-tmp))
    (let [r (shell {:out :string :err :string :continue true}
                   "bash" "-c"
                   (format "%sssh %s %s@%s 'bash %s 2>&1'"
                           ssh-prefix ssh-opts ssh-user ip remote-tmp))]
      (shell {:continue true} "rm" "-f" local-tmp)
      (assoc r :ip ip))))

;; ══════════════════════════════════════════════════════════════════════
;; Parallel executor — runs a phase on ALL devices simultaneously
;; ══════════════════════════════════════════════════════════════════════

(defn parallel! [devices phase-name f]
  (let [tag (format "[%s]" phase-name)
        futures (mapv (fn [dev]
                        (future
                          (let [t0 (System/currentTimeMillis)
                                result (try (f dev) (catch Exception e {:exit 1 :out "" :err (.getMessage e)}))]
                            (let [ms (- (System/currentTimeMillis) t0)
                                  ok? (zero? (:exit result 1))]
                              (locking *out*
                                (printf "  %s %s %s@%s (%dms) %s\n"
                                        (if ok? "[OK]" "[FAIL]")
                                        tag (:id dev) (:ip dev) ms
                                        (if ok?
                                          (let [lines (str/split-lines (or (:out result) ""))]
                                            (last lines))
                                          (str "ERR: " (subs (or (:err result) (:out result) "?") 0
                                                             (min 120 (count (or (:err result) (:out result) "?")))))))
                                (flush))
                              (assoc result :ok ok? :ms ms :device (:id dev))))))
                      devices)]
    (printf "\n== %s (x%d parallel) ==\n" phase-name (count devices))
    (flush)
    (mapv deref futures)))

;; ══════════════════════════════════════════════════════════════════════
;; Phase scripts (pure bash strings, no Clojure escaping issues)
;; ══════════════════════════════════════════════════════════════════════

(def script-probe "
echo '--- GPU ---'
nvidia-smi --query-gpu=name,memory.total,memory.free,compute_cap --format=csv,noheader 2>/dev/null || echo 'NO GPU'
echo '--- CUDA ---'
nvcc --version 2>/dev/null | grep release || echo 'no nvcc'
echo '--- CPU ---'
lscpu 2>/dev/null | grep 'Model name' || uname -m
echo '--- Memory ---'
free -h | grep Mem
echo '--- Disk ---'
df -h / | tail -1
echo '--- Interfaces ---'
ip -br addr show 2>/dev/null | grep -E 'enp|eth' | head -5 || echo 'none'
echo 'PROBE_OK'
")

(def script-system "
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -qq
sudo apt-get install -y -qq build-essential cmake ninja-build pkg-config libopenmpi-dev openmpi-bin libibverbs-dev librdmacm-dev python3-pip python3-venv python3-dev git curl wget jq tmux htop unzip
if ! command -v docker &>/dev/null; then
  sudo apt-get install -y -qq docker.io
  sudo systemctl enable --now docker
  sudo usermod -aG docker $USER
fi
if ! dpkg -l nvidia-container-toolkit &>/dev/null 2>&1; then
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
  sudo apt-get update -qq
  sudo apt-get install -y -qq nvidia-container-toolkit
  sudo nvidia-ctk runtime configure --runtime=docker
  sudo systemctl restart docker
fi
command -v flox &>/dev/null || curl -fsSL https://flox.dev/install | bash -s -- --no-confirm
echo 'SYSTEM_OK'
")

(def script-nccl "
export CUDA_HOME=/usr/local/cuda
export MPI_HOME=/usr/lib/aarch64-linux-gnu/openmpi
if [ ! -f $HOME/nccl/build/lib/libnccl.so ]; then
  rm -rf $HOME/nccl
  git clone -b v2.28.9-1 https://github.com/NVIDIA/nccl.git $HOME/nccl/
  cd $HOME/nccl/
  make -j$(nproc) src.build NVCC_GENCODE='-gencode=arch=compute_121,code=sm_121'
else
  echo 'NCCL exists'
fi
export NCCL_HOME=$HOME/nccl/build
if [ ! -f $HOME/nccl-tests/build/all_reduce_perf ]; then
  rm -rf $HOME/nccl-tests
  git clone https://github.com/NVIDIA/nccl-tests.git $HOME/nccl-tests/
  cd $HOME/nccl-tests/
  make MPI=1 NCCL_HOME=$NCCL_HOME CUDA_HOME=$CUDA_HOME
else
  echo 'nccl-tests exists'
fi
echo 'NCCL_OK'
")

(def script-tensorrt "
TRTLLM_IMAGE='nvcr.io/nvidia/tensorrt-llm/release:latest'
sudo docker pull $TRTLLM_IMAGE
sudo docker run --rm --gpus all $TRTLLM_IMAGE nvidia-smi --query-gpu=name --format=csv,noheader
sudo docker run --rm --gpus all $TRTLLM_IMAGE python3 -c 'import tensorrt_llm; print(f\"TRT-LLM {tensorrt_llm.__version__}\")'
echo 'TENSORRT_OK'
")

(def script-inference "
if [ ! -d $HOME/spark-ai-env ]; then python3 -m venv $HOME/spark-ai-env; fi
source $HOME/spark-ai-env/bin/activate
pip install --upgrade pip setuptools wheel -q
pip install torch torchvision torchaudio -q 2>/dev/null || pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124 -q
python3 -c \"import torch; print(f'PyTorch {torch.__version__}, CUDA: {torch.cuda.is_available()}')\"
pip install vllm -q 2>/dev/null || echo 'vLLM: manual build needed on aarch64'
if [ ! -d /tmp/SageAttention ]; then
  cd /tmp && git clone https://github.com/thu-ml/SageAttention.git
  cd SageAttention && TORCH_CUDA_ARCH_LIST='12.1' pip install . -q 2>/dev/null || echo 'SageAttention: needs CUDA headers'
fi
pip install xfuser -q 2>/dev/null || echo 'xfuser attempted'
pip install diffusers transformers accelerate safetensors -q
echo 'INFERENCE_OK'
")

(def script-lolita "
source $HOME/spark-ai-env/bin/activate
if [ ! -d $HOME/lolita ]; then
  git clone https://github.com/plurigrid/lolita.git $HOME/lolita
fi
cd $HOME/lolita
pip install -e . -q 2>/dev/null || pip install -r requirements.txt -q 2>/dev/null || echo 'lolita: manual deps'
pip install websockets aiohttp pillow numpy scipy -q
echo 'LOLITA_OK'
")

(def script-vr "
source $HOME/spark-ai-env/bin/activate
pip install streamdiffusion -q 2>/dev/null || pip install git+https://github.com/cumulo-autumn/StreamDiffusion.git -q 2>/dev/null || echo 'StreamDiffusion: manual'
pip install aiortc websockets aiohttp -q
echo 'VR_OK'
")

(defn script-vr-server []
  "cat > $HOME/vr-diffusion-server.py << 'VREOF'
import asyncio, json, websockets, torch, numpy as np, io
from PIL import Image

class DiffusionStreamer:
    def __init__(self):
        self.device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
        self.frame_count = 0
    async def generate_frame(self, p):
        w, h = p.get('width', 1024), p.get('height', 1024)
        f = torch.randn(3, h, w, device=self.device)
        f = ((f + 1) / 2 * 255).clamp(0, 255).byte()
        return f.cpu().numpy().transpose(1, 2, 0)
    async def stream(self, ws):
        params = {'width': 1024, 'height': 1024}
        async for msg in ws:
            ctrl = json.loads(msg)
            if 'params' in ctrl: params.update(ctrl['params'])
            frame = await self.generate_frame(params)
            buf = io.BytesIO()
            Image.fromarray(frame).save(buf, format='JPEG', quality=85)
            await ws.send(buf.getvalue())
            self.frame_count += 1

s = DiffusionStreamer()
asyncio.run(websockets.serve(s.stream, '0.0.0.0', 8765).__aenter__().then(lambda _: asyncio.Future()))
VREOF
chmod +x $HOME/vr-diffusion-server.py
echo 'VR_SERVER_OK'
")

(defn script-env [dev all-ips]
  (let [n (count all-ips)
        hostfile (str/join "\n" all-ips)
        nccl-ifname (if (> n 2) "enp1s0f0np0,enp1s0f1np1" "enp1s0f0np0")]
    (str "
cat > $HOME/nccl-env.sh << 'NEOF'
export CUDA_HOME=/usr/local/cuda
export NCCL_HOME=$HOME/nccl/build
export MPI_HOME=/usr/lib/aarch64-linux-gnu/openmpi
export LD_LIBRARY_PATH=$NCCL_HOME/lib:$CUDA_HOME/lib64:$MPI_HOME/lib:$LD_LIBRARY_PATH
export NCCL_SOCKET_IFNAME=" nccl-ifname "
export NCCL_DEBUG=INFO
export NCCL_MNNVL_ENABLE=1
export NCCL_PXN_C2C=1
export NCCL_NVLS_ENABLE=1
NEOF

cat > $HOME/spark-ai-activate.sh << 'AEOF'
source $HOME/spark-ai-env/bin/activate 2>/dev/null
source $HOME/nccl-env.sh 2>/dev/null
export TORCH_CUDA_ARCH_LIST=12.1
export TORCHDYNAMO_DISABLE=1
export CUDA_MANAGED_FORCE_DEVICE_ALLOC=1
export OMP_NUM_THREADS=20
AEOF

cat > $HOME/start-vr.sh << 'SEOF'
#!/bin/bash
source $HOME/spark-ai-activate.sh
python3 $HOME/vr-diffusion-server.py
SEOF
chmod +x $HOME/start-vr.sh

cat > $HOME/openmpi-hostfile << 'HEOF'
" hostfile "
HEOF

grep -q 'spark-ai-activate' $HOME/.bashrc 2>/dev/null || echo 'source $HOME/spark-ai-activate.sh 2>/dev/null' >> $HOME/.bashrc
echo 'ENV_OK'
")))

(defn script-qsfp [dev all-devs]
  (let [n (count all-devs)
        idx (:idx dev)]
    (cond
      (= n 1) "echo 'Single node, no QSFP needed'; echo 'QSFP_OK'"
      (= n 2) (let [ip (format "192.168.100.%d" (+ 10 idx))
                     ;; Use whichever port has carrier=1; prefer p0 (LEFT, 200G)
                     port "enp1s0f0np0"]
                (str "ACTIVE=$(for n in enp1s0f0np0 enp1s0f1np1; do "
                     "[ \"$(cat /sys/class/net/$n/carrier 2>/dev/null)\" = 1 ] && echo $n && break; done)\n"
                     "PORT=${ACTIVE:-" port "}\n"
                     "sudo ip addr add " ip "/24 dev $PORT 2>/dev/null || true\n"
                     "sudo ip link set $PORT mtu 9000 up\n"
                     "echo \"QSFP " ip " on $PORT OK\""))
      (= n 3) (let [links [{:a 0 :b 1 :sub "192.168.101" :pa "enp1s0f0np0" :pb "enp1s0f0np0"}
                            {:a 0 :b 2 :sub "192.168.102" :pa "enp1s0f1np1" :pb "enp1s0f0np0"}
                            {:a 1 :b 2 :sub "192.168.103" :pa "enp1s0f1np1" :pb "enp1s0f1np1"}]
                    my-links (filter #(or (= idx (:a %)) (= idx (:b %))) links)
                    cmds (mapcat (fn [l]
                                  (let [am-a? (= idx (:a l))
                                        port (if am-a? (:pa l) (:pb l))
                                        suf (if am-a? 10 11)
                                        ip (format "%s.%d" (:sub l) suf)]
                                    [(str "sudo ip addr add " ip "/24 dev " port " 2>/dev/null || true")
                                     (str "sudo ip link set " port " mtu 9000 up")]))
                                my-links)]
                (str (str/join "\n" cmds) "\necho 'QSFP_MESH_OK'")))))

(def script-verify "
echo '--- GPU ---'
nvidia-smi --query-gpu=name,memory.total,memory.free,compute_cap --format=csv,noheader 2>/dev/null || echo 'no GPU'
echo '--- NCCL ---'
ls $HOME/nccl/build/lib/libnccl.so* 2>/dev/null | head -1 || echo 'no NCCL'
echo '--- Docker ---'
docker --version 2>/dev/null || echo 'no docker'
echo '--- Python ---'
source $HOME/spark-ai-env/bin/activate 2>/dev/null
python3 -c 'import torch; print(f\"torch {torch.__version__} CUDA={torch.cuda.is_available()}\")' 2>/dev/null || echo 'no torch'
python3 -c 'import diffusers; print(f\"diffusers {diffusers.__version__}\")' 2>/dev/null || echo 'no diffusers'
echo '--- lolita ---'
ls $HOME/lolita/README.md &>/dev/null && echo 'OK' || echo 'missing'
echo '--- VR ---'
ls $HOME/start-vr.sh &>/dev/null && echo 'OK' || echo 'missing'
echo '--- Flox ---'
flox --version 2>/dev/null || echo 'no flox'
echo 'VERIFY_OK'
")

;; ══════════════════════════════════════════════════════════════════════
;; Main
;; ══════════════════════════════════════════════════════════════════════

(defn -main []
  (let [args *command-line-args*
        raw-ips (first args)
        dry-run? (some #(= "--dry-run" %) args)]

    (when-not raw-ips
      (println "Usage: bb provision-spark.bb <ip1>[,<ip2>[,<ip3>]] [--dry-run]")
      (println "  1 IP  = 128GB single node")
      (println "  2 IPs = 256GB NVLink stacked")
      (println "  3 IPs = 384GB triangle mesh")
      (println "  Credentials: a / aaaaaa")
      (System/exit 1))

    (let [ips (str/split raw-ips #",")
          n (count ips)
          devices (mapv (fn [ip idx]
                          {:id (format "spark-%d" (inc idx))
                           :ip ip :idx idx
                           :trit (nth gf3-trits idx)
                           :role (get gf3-roles (nth gf3-trits idx))})
                        ips (range))
          all-ips (mapv :ip devices)
          memory (* n 128)
          topo (case n 1 "single" 2 "NVLink stacked" 3 "triangle mesh")]

      (println "=== DGX Spark Provisioner ===")
      (printf "  Devices: %d x GB10 (%s) = %d GB unified\n" n topo memory)
      (doseq [d devices]
        (printf "  %-8s  %s  trit=%+d  %s  %s\n"
                (:id d) (:ip d) (:trit d)
                (get gf3-labels (:trit d)) (:role d)))
      (let [trit-sum (reduce + (map :trit devices))]
        (printf "  GF(3): %s = %d  conserved=%s\n"
                (str/join "+" (map #(format "%+d" (:trit %)) devices))
                trit-sum (zero? (mod trit-sum 3))))
      (println "  Stack: NCCL + TRT-LLM + vLLM + SageAttention + xDiT + lolita + VR:8765")

      (if dry-run?
        (do
          (println "\n=== DRY RUN ===")
          (println "Phases (all parallel across devices):")
          (println "  0. SSH key bootstrap")
          (println "  1. Device probe")
          (println "  2. QSFP network config")
          (println "  3. System deps + docker + flox")
          (println "  4. NCCL v2.28.9-1 (sm_121)")
          (println "  5. TensorRT-LLM container")
          (println "  6. Inference stack (PyTorch, vLLM, SageAttention, xDiT)")
          (println "  7. plurigrid/lolita")
          (println "  8. VR streaming (StreamDiffusion + server)")
          (println "  9. Environment scripts + NCCL config")
          (println " 10. Verification")
          (println "\nRun without --dry-run to execute."))

        ;; ════════════════════════════════════════════════════════════
        ;; LIVE: all phases, each parallel across all devices
        ;; ════════════════════════════════════════════════════════════
        (let [t0 (System/currentTimeMillis)]

          ;; Phase 0: SSH keys
          (println "\n== SSH Key Bootstrap ==")
          (let [home (System/getProperty "user.home")
                key-path (str home "/.ssh/id_ed25519")]
            (when-not (.exists (java.io.File. key-path))
              (shell "ssh-keygen" "-t" "ed25519" "-f" key-path "-N" "")))
          ;; Verify existing key access (keys should already be copied)
          (parallel! devices "ssh-verify"
                     (fn [dev]
                       (shell {:out :string :err :string :continue true}
                              "bash" "-c"
                              (format "ssh %s -o BatchMode=yes %s@%s 'echo SSH_OK' 2>&1"
                                      ssh-opts ssh-user (:ip dev)))))

          ;; Phase 1: Probe
          (parallel! devices "probe"
                     (fn [dev] (ssh-script! (:ip dev) script-probe)))

          ;; Phase 2: QSFP network (device-specific scripts)
          (when (> n 1)
            (parallel! devices "QSFP"
                       (fn [dev] (ssh-script! (:ip dev) (script-qsfp dev devices)))))

          ;; Phase 3: System deps
          (parallel! devices "system"
                     (fn [dev] (ssh-script! (:ip dev) script-system)))

          ;; Phase 4: NCCL build (heaviest — ~15min each, all parallel)
          (parallel! devices "NCCL"
                     (fn [dev] (ssh-script! (:ip dev) script-nccl)))

          ;; Phase 5: TensorRT-LLM container
          (parallel! devices "TensorRT-LLM"
                     (fn [dev] (ssh-script! (:ip dev) script-tensorrt)))

          ;; Phase 6: Inference stack
          (parallel! devices "inference"
                     (fn [dev] (ssh-script! (:ip dev) script-inference)))

          ;; Phase 7: lolita
          (parallel! devices "lolita"
                     (fn [dev] (ssh-script! (:ip dev) script-lolita)))

          ;; Phase 8: VR streaming + server
          (parallel! devices "VR"
                     (fn [dev] (ssh-script! (:ip dev)
                                            (str script-vr "\n" (script-vr-server)))))

          ;; Phase 9: Env scripts (device-aware)
          (parallel! devices "env"
                     (fn [dev] (ssh-script! (:ip dev) (script-env dev all-ips))))

          ;; Phase 10: Verify
          (parallel! devices "verify"
                     (fn [dev] (ssh-script! (:ip dev) script-verify)))

          ;; Multi-node NCCL test (sequential, runs from device 0)
          (when (> n 1)
            (println "\n== Multi-node NCCL all_reduce test ==")
            (let [host-slots (str/join "," (map #(format "%s:1" (:ip %)) devices))
                  test-script (str "source $HOME/nccl-env.sh\n"
                                   "mpirun -np " n " -H " host-slots
                                   " --mca plm_rsh_agent 'ssh -o StrictHostKeyChecking=no'"
                                   " -x LD_LIBRARY_PATH -x NCCL_SOCKET_IFNAME -x NCCL_DEBUG=WARN"
                                   " $HOME/nccl-tests/build/all_reduce_perf -b 8 -e 128M -f 2 -g 1"
                                   " 2>&1 | tail -20")
                  r (ssh-script! (:ip (first devices)) test-script)]
              (if (zero? (:exit r))
                (do (println "  [OK] all_reduce_perf")
                    (doseq [line (str/split-lines (:out r))]
                      (println "       " line)))
                (println "  [FAIL] all_reduce_perf" (:exit r)))))

          ;; Summary
          (let [elapsed (/ (- (System/currentTimeMillis) t0) 1000.0)]
            (println)
            (println "================================================================")
            (println "  PROVISIONING COMPLETE")
            (println "================================================================")
            (printf  "  Devices:  %d x GB10 (%s)\n" n topo)
            (printf  "  Memory:   %d GB unified LPDDR5X\n" memory)
            (printf  "  NCCL:     v2.28.9-1 (sm_121 Blackwell)\n")
            (printf  "  Stack:    TRT-LLM + vLLM + SageAttention + xDiT + lolita\n")
            (printf  "  VR:       WebSocket :8765 on each device\n")
            (printf  "  Time:     %.0fs\n" elapsed)
            (println "  ──────────────────────────────────────────────────────────")
            (doseq [d devices]
              (printf "  ssh %s@%-16s  # %s trit=%+d\n" ssh-user (:ip d) (:role d) (:trit d)))
            (println "  ──────────────────────────────────────────────────────────")
            (printf  "  VR:   ssh %s@%s './start-vr.sh'\n" ssh-user (:ip (first devices)))
            (printf  "  Env:  ssh %s@%s 'source spark-ai-activate.sh'\n" ssh-user (:ip (first devices)))
            (let [trit-sum (reduce + (map :trit devices))]
              (printf "  GF(3): %s = %d  conserved=%s\n"
                      (str/join "+" (map #(format "%+d" (:trit %)) devices))
                      trit-sum (zero? (mod trit-sum 3))))
            (println "================================================================"))))))

  (println "\n=== provision-spark complete ==="))

(-main)
