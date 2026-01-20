#!/usr/bin/env bb
;; Build HaikuOS ARM64 image from source
;; Requires: Linux host, ~20GB disk space, ~4GB RAM
;;
;; Usage:
;;   bb build-haiku-arm64.bb
;;   # or in Docker:
;;   docker run -it --rm -v $(pwd):/work ubuntu:22.04 bash -c "apt-get update && apt-get install -y curl && curl -sL https://raw.githubusercontent.com/babashka/babashka/master/install | bash && cd /work && bb build-haiku-arm64.bb"

(require '[babashka.process :as p]
         '[babashka.fs :as fs]
         '[clojure.string :as str])

(def build-dir (or (System/getenv "BUILD_DIR") "/tmp/haiku-build"))
(def output-dir (or (System/getenv "OUTPUT_DIR") "."))

(defn sh! [& args]
  (let [cmd (str/join " " args)]
    (println ">>> " cmd)
    (let [result (p/shell {:out :inherit :err :inherit} cmd)]
      (when (not= 0 (:exit result))
        (throw (ex-info (str "Command failed: " cmd) {:exit (:exit result)}))))))

(defn sh-ok? [& args]
  (try
    (let [result (p/shell {:out :string :err :string} (str/join " " args))]
      (= 0 (:exit result)))
    (catch Exception _ false)))

(defn install-deps-debian []
  (println "=== Installing build dependencies ===")
  (sh! "apt-get update")
  (sh! "apt-get install -y"
       "git g++ bison flex libz-dev autoconf automake libtool"
       "gawk nasm wget curl python3 mtools dosfstools bc xorriso"))

(defn clone-repos []
  (fs/create-dirs build-dir)
  
  (when-not (fs/exists? (str build-dir "/haiku"))
    (println "=== Cloning Haiku source ===")
    (sh! "git clone --depth 1 https://review.haiku-os.org/haiku" (str build-dir "/haiku")))
  
  (when-not (fs/exists? (str build-dir "/buildtools"))
    (println "=== Cloning buildtools ===")
    (sh! "git clone --depth 1 https://review.haiku-os.org/buildtools" (str build-dir "/buildtools"))))

(defn build-toolchain []
  (let [gen-dir (str build-dir "/haiku/generated.arm64")]
    (fs/create-dirs gen-dir)
    (println "=== Building ARM64 cross-compiler (this takes ~30 minutes) ===")
    (p/shell {:dir gen-dir :out :inherit :err :inherit}
             "../configure" "-j4"
             "--cross-tools-source" "../../buildtools"
             "--build-cross-tools" "arm64")))

(defn build-image []
  (let [gen-dir (str build-dir "/haiku/generated.arm64")]
    (println "=== Building HaikuOS ARM64 MMC image ===")
    (p/shell {:dir gen-dir :out :inherit :err :inherit}
             "jam" "-j4" "-q" "@minimum-mmc")))

(defn copy-output []
  (let [src (str build-dir "/haiku/generated.arm64/haiku-mmc.image")
        dst (str output-dir "/haiku-arm64-mmc.img")]
    (if (fs/exists? src)
      (do
        (fs/copy src dst {:replace-existing true})
        (println "=== Build successful! ===")
        (println "Image saved to:" dst)
        (println "")
        (println "To use with boxxy:")
        (println "  1. Convert to raw format if needed")
        (println "  2. Use with cmd/haiku-gui/main.go"))
      (do
        (println "=== Build failed - no image produced ===")
        (println "The ARM64 port may be too incomplete to build.")
        (println "Check: https://www.haiku-os.org/guides/building/port_status/")
        (System/exit 1)))))

(defn -main []
  (println "=== HaikuOS ARM64 Build Script ===")
  (println "This will build an ARM64 image for Apple Virtualization.framework")
  (println "")
  (println "Build dir:" build-dir)
  (println "Output dir:" output-dir)
  (println "")
  
  ;; Check if we're on Linux
  (when-not (str/includes? (System/getProperty "os.name") "Linux")
    (println "WARNING: This script is designed for Linux.")
    (println "macOS builds fail due to toolchain incompatibilities.")
    (println "Consider running in Docker or a Linux VM.")
    (println ""))
  
  ;; Install deps if apt-get available
  (when (sh-ok? "which apt-get")
    (install-deps-debian))
  
  (clone-repos)
  (build-toolchain)
  (build-image)
  (copy-output))

(-main)
