#!/usr/bin/env bb
;; infect-dgx.bb — Set up flox agentic development environments on ASUS DGX machines
;;
;; "Infect" means: install flox, pull bmorphism/effective-topos environment,
;; set up the topos-gateway, and register as resource-sharing nodes.
;;
;; Prerequisites:
;;   - Tailscale mesh: both DGX machines on pirate-dragon tailnet
;;   - SSH access via Tailscale: ssh dgx-alpha.pirate-dragon.ts.net
;;   - The machines have NVIDIA GPUs and CUDA installed

(ns infect-dgx
  (:require [babashka.process :refer [shell process]]
            [clojure.string :as str]))

;; ══════════════════════════════════════════════════════════════════════
;; DGX Machine Registry
;; ══════════════════════════════════════════════════════════════════════

(def dgx-machines
  [{:name "dgx-alpha"
    :host "dgx-alpha.pirate-dragon.ts.net"
    :trit 1    ; PLUS — generator (primary compute)
    :role :generator
    :gpus "NVIDIA A100 ×8"
    :models ["llama-3.3-405b" "deepseek-v3"]}

   {:name "dgx-beta"
    :host "dgx-beta.pirate-dragon.ts.net"
    :trit -1   ; MINUS — validator (verification/audit)
    :role :validator
    :gpus "NVIDIA A100 ×8"
    :models ["deepseek-r1" "qwen-72b"]}])

;; ══════════════════════════════════════════════════════════════════════
;; Flox Environment Manifest for DGX
;; ══════════════════════════════════════════════════════════════════════

(def topos-manifest
  "## Flox Environment Manifest — topos (DGX agentic development)
##
## This is the \"topos\" environment: our Portkey replacement.
## It provides the full effective-topos toolchain plus:
##   - AI inference servers (vLLM, TGI)
##   - Gateway routing (topos-gateway)
##   - Observability (Prometheus + Grafana)
##   - KMS integration (age encryption + sideref tokens)
##
version = 1

[install]
## ═══════════════════════════════════════════════════════════════════
## Core Toolchain (from effective-topos)
## ═══════════════════════════════════════════════════════════════════
babashka.pkg-path = \"babashka\"
gh.pkg-path = \"gh\"
jq.pkg-path = \"jq\"
tmux.pkg-path = \"tmux\"
curl.pkg-path = \"curl\"
git.pkg-path = \"git\"
age.pkg-path = \"age\"
nats-server.pkg-path = \"nats-server\"
nats-cli.pkg-path = \"natscli\"

## ═══════════════════════════════════════════════════════════════════
## Language Runtimes
## ═══════════════════════════════════════════════════════════════════
python3.pkg-path = \"python3\"
nodejs.pkg-path = \"nodejs\"
go.pkg-path = \"go\"

## ═══════════════════════════════════════════════════════════════════
## AI Inference
## ═══════════════════════════════════════════════════════════════════
# vLLM and TGI are installed via pip in the activation hook
# because they need CUDA-specific wheels

## ═══════════════════════════════════════════════════════════════════
## Observability
## ═══════════════════════════════════════════════════════════════════
prometheus.pkg-path = \"prometheus\"

## ═══════════════════════════════════════════════════════════════════
## Security / KMS
## ═══════════════════════════════════════════════════════════════════
# age: file encryption (replaces GPG for key management)
# sops: secret management with age keys
sops.pkg-path = \"sops\"

[vars]
TOPOS_ROLE = \"gateway\"
TOPOS_TRIT = \"0\"
NATS_URL = \"nats://localhost:4222\"
GAY_SEED = \"0x285508656870f24a\"

[hook]
on-activate = '''
  echo \"═══════════════════════════════════════════════════════\"
  echo \"  topos environment activated\"
  echo \"  Role: $TOPOS_ROLE  Trit: $TOPOS_TRIT\"
  echo \"═══════════════════════════════════════════════════════\"

  # Install vLLM if CUDA is available
  if command -v nvidia-smi &>/dev/null; then
    echo \"CUDA detected. Checking vLLM...\"
    if ! python3 -c 'import vllm' 2>/dev/null; then
      echo \"Installing vLLM...\"
      pip install vllm --quiet
    fi
    echo \"GPU status:\"
    nvidia-smi --query-gpu=name,memory.total,memory.free --format=csv,noheader
  fi

  # Start NATS if not running
  if ! pgrep -x nats-server > /dev/null; then
    echo \"Starting NATS server...\"
    nats-server -p 4222 -m 8222 &
    disown
  fi
'''

[services]
# NATS messaging server
nats.command = \"nats-server -p 4222 -m 8222\"

[profile]
# Quick aliases
topos-gateway.command = \"bb worlds/b/basin/topos-gateway.bb\"
topos-status.command = \"echo \\\"Role: $TOPOS_ROLE  Trit: $TOPOS_TRIT\\\" && nvidia-smi --query-gpu=utilization.gpu --format=csv,noheader 2>/dev/null || echo 'No GPU'\"
")

;; ══════════════════════════════════════════════════════════════════════
;; Infection Steps
;; ══════════════════════════════════════════════════════════════════════

(defn ssh-cmd
  "Build SSH command for a DGX machine."
  [host cmd]
  (format "ssh -o StrictHostKeyChecking=no %s '%s'" host cmd))

(defn step-install-flox
  "Step 1: Install flox on the DGX machine."
  [machine]
  (println (format "  [%s] Step 1: Install flox..." (:name machine)))
  {:step "install-flox"
   :cmd (ssh-cmd (:host machine)
                 "curl -fsSL https://flox.dev/install | bash -s -- --no-confirm")
   :dry-run true})

(defn step-create-topos-env
  "Step 2: Create the topos flox environment."
  [machine]
  (println (format "  [%s] Step 2: Create topos environment..." (:name machine)))
  {:step "create-topos-env"
   :cmd (ssh-cmd (:host machine)
                 (str "mkdir -p ~/.topos/.flox/env && "
                      "cat > ~/.topos/.flox/env/manifest.toml << 'MANIFEST'\n"
                      topos-manifest
                      "\nMANIFEST"))
   :dry-run true})

(defn step-configure-role
  "Step 3: Configure the machine's GF(3) role."
  [machine]
  (println (format "  [%s] Step 3: Configure role %s (trit=%+d)..."
                   (:name machine) (name (:role machine)) (:trit machine)))
  {:step "configure-role"
   :cmd (ssh-cmd (:host machine)
                 (format (str "sed -i 's/TOPOS_ROLE = .*/TOPOS_ROLE = \"%s\"/' ~/.topos/.flox/env/manifest.toml && "
                              "sed -i 's/TOPOS_TRIT = .*/TOPOS_TRIT = \"%d\"/' ~/.topos/.flox/env/manifest.toml")
                         (name (:role machine)) (:trit machine)))
   :dry-run true})

(defn step-setup-kms
  "Step 4: Set up KMS with age encryption + sideref tokens."
  [machine]
  (println (format "  [%s] Step 4: Set up KMS (age + sideref)..." (:name machine)))
  {:step "setup-kms"
   :cmd (ssh-cmd (:host machine)
                 (str "mkdir -p ~/.topos/kms && "
                      "if [ ! -f ~/.topos/kms/identity.age ]; then "
                      "  age-keygen -o ~/.topos/kms/identity.age 2>/dev/null; "
                      "  age-keygen -y ~/.topos/kms/identity.age > ~/.topos/kms/recipient.pub; "
                      "  echo 'KMS identity created'; "
                      "else "
                      "  echo 'KMS identity exists'; "
                      "fi"))
   :dry-run true})

(defn step-register-provider
  "Step 5: Register as a topos-gateway provider."
  [machine]
  (println (format "  [%s] Step 5: Register as gateway provider..." (:name machine)))
  {:step "register-provider"
   :config {:provider (:name machine)
            :endpoint (format "http://%s:8080/v1" (:host machine))
            :trit (:trit machine)
            :role (:role machine)
            :models (:models machine)
            :gpus (:gpus machine)}
   :dry-run true})

(defn step-activate
  "Step 6: Activate the topos environment."
  [machine]
  (println (format "  [%s] Step 6: Activate topos environment..." (:name machine)))
  {:step "activate"
   :cmd (ssh-cmd (:host machine) "cd ~/.topos && flox activate -- echo 'topos activated'")
   :dry-run true})

;; ══════════════════════════════════════════════════════════════════════
;; Full Infection Pipeline
;; ══════════════════════════════════════════════════════════════════════

(defn infect!
  "Infect a DGX machine with the topos environment."
  [machine]
  (println (format "\n══ Infecting %s (%s, trit=%+d) ══"
                   (:name machine) (:gpus machine) (:trit machine)))
  (let [steps [(step-install-flox machine)
               (step-create-topos-env machine)
               (step-configure-role machine)
               (step-setup-kms machine)
               (step-register-provider machine)
               (step-activate machine)]]
    {:machine (:name machine)
     :steps steps
     :status :dry-run}))

;; ══════════════════════════════════════════════════════════════════════
;; Main
;; ══════════════════════════════════════════════════════════════════════

(defn -main []
  (println "=== DGX Infection: topos flox agentic environment ===")
  (println)
  (println "Machines:")
  (doseq [m dgx-machines]
    (printf "  %-12s  %s  trit=%+d  role=%-10s  models=%s%n"
            (:name m) (:host m) (:trit m) (name (:role m))
            (str/join ", " (:models m))))

  (println)
  (println "GF(3) balance check:")
  (let [trits (map :trit dgx-machines)
        sum (reduce + trits)
        ;; Add the local machine (ERGODIC coordinator, trit=0)
        total-with-local sum]
    (printf "  DGX trits: %s = %d%n" (str/join " + " (map #(format "%+d" %) trits)) sum)
    (printf "  + local coordinator (trit=0) = %d%n" total-with-local)
    (printf "  residue = %d  conserved = %s%n"
            (mod (+ (mod total-with-local 3) 3) 3)
            (zero? (mod total-with-local 3))))

  ;; Infect both machines
  (let [results (mapv infect! dgx-machines)]
    (println)
    (println "=== Infection Summary ===")
    (doseq [r results]
      (printf "  %s: %d steps, status=%s%n"
              (:machine r) (count (:steps r)) (name (:status r))))
    (println)
    (println "All commands are DRY RUN. To execute for real:")
    (println "  1. Ensure Tailscale is connected: tailscale status")
    (println "  2. Verify SSH access: ssh dgx-alpha.pirate-dragon.ts.net whoami")
    (println "  3. Run with --execute flag (not yet implemented)")
    (println)
    (println "Once infected, the DGX machines become topos-gateway providers.")
    (println "The local machine (ERGODIC, trit=0) routes between them.")
    (println "GF(3) conservation is maintained: +1 + -1 + 0 = 0 ✓"))

  (println "\n=== DGX infection plan complete ==="))

(-main)
