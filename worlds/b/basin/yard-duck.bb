#!/usr/bin/env bb
;; yard-duck.bb — YARD documentation support for .duck domains in basin
;;
;; Serves DuckDB-backed documentation via babashka HTTP server inside
;; a boxxy x86_64 Linux guest (Rosetta) or directly on macOS host.
;;
;; Architecture:
;;   Pi-hole dnsmasq: *.duck -> 127.0.0.1 (containment boundary)
;;   This server:     :80 -> parse request -> DuckDB query -> HTML response
;;   DuckDB files:    ~/i/duck/*.duckdb (20+ databases, ~40MB total)
;;
;; Breadcrumb navigation:
;;   basin.duck/                           -> list all .duckdb files
;;   basin.duck/basin-replay               -> tables in basin-replay.duckdb
;;   basin.duck/basin-replay/skills        -> rows in skills table
;;   basin.duck/basin-replay/skills?limit=10&offset=0 -> paginated
;;
;; GF(3) Assignment:
;;   YARD server    -> trit  0 (ERGODIC/coordinator)
;;   DuckDB queries -> trit -1 (MINUS/validator)
;;   HTML generator -> trit +1 (PLUS/generator)
;;   Sum: 0 + (-1) + 1 = 0 CONSERVED
;;
;; References:
;;   - Sucholutsky et al. 2310.13018 "Getting aligned on representational alignment"
;;     Framework: systems/stimuli/representations/similarity/objective
;;     In .duck YARD: system=DuckDB, stimuli=breadcrumb path, representation=query result,
;;     similarity=schema match, objective=measuring (hallucination benchmark)
;;   - Robitaille, flowmaps: "interactive introspection" > observability
;;     .duck YARD = introspectable data flow (not just static docs)
;;   - Capucci, categorical cybernetics: controllable process = YARD server,
;;     control process = breadcrumb navigation, cube = schema evolution
;;   - Apple ParavirtualizedGraphics + Metal 4 TBDR:
;;     PGDevice -> Metal-accelerated tile-based deferred rendering in macOS guest
;;     Tile memory = shared representation space (imageblocks persist across draws)
;;     Raster order groups = fragment thread synchronization (GF(3) ordering)
;;     Metal 4 triple-buffered frames: encode(+1)/execute(0)/display(-1) = GF(3) rotation
;;     MTL4CommandBuffer reusable + MTL4CommandAllocator per frame = allocator rotation
;;     Linux guests: VirtioGPU (no tile memory); headless OK for DuckDB serving
;;
;; Usage:
;;   bb yard-duck.bb serve                   # Start HTTP server on :8069
;;   bb yard-duck.bb serve --port 80         # Production port (needs root in VM)
;;   bb yard-duck.bb query <db> <sql>        # Direct DuckDB query
;;   bb yard-duck.bb list                    # List available .duckdb files
;;   bb yard-duck.bb schema <db>             # Show schema for database

(ns yard-duck
  (:require [babashka.process :refer [shell]]
            [clojure.string :as str]
            [org.httpkit.server :as http]
            [cheshire.core :as json]
            [babashka.fs :as fs]))

;; ══════════════════════════════════════════════════════════════════════
;; Configuration
;; ══════════════════════════════════════════════════════════════════════

(def PORT (or (some-> (System/getenv "YARD_PORT") parse-long) 8069))
(def DUCK_DIR (or (System/getenv "DUCK_DIR")
                  (str (System/getProperty "user.home") "/i/duck")))
(def PAGE_SIZE 50)

;; GF(3) trit assignments for YARD operations
(def TRIT_MINUS -1)   ;; Query/read (contraction)
(def TRIT_ERGODIC 0)  ;; Server/route (coordination)
(def TRIT_PLUS +1)    ;; Generate HTML (expansion)

;; ══════════════════════════════════════════════════════════════════════
;; DuckDB Integration
;; ══════════════════════════════════════════════════════════════════════

(defn duck-query
  "Execute SQL against a .duckdb file. Returns {:ok true :data [...]} or {:ok false :error ...}"
  [db-path sql]
  (let [result (shell {:out :string :err :string :continue true}
                      "duckdb" db-path "-json" "-c" sql)]
    (if (= 0 (:exit result))
      {:ok true
       :trit TRIT_MINUS
       :data (try (json/parse-string (:out result) true)
                  (catch Exception _ (:out result)))}
      {:ok false :error (:err result)})))

(defn list-databases
  "List all .duckdb files in DUCK_DIR"
  []
  (->> (fs/glob DUCK_DIR "*.duckdb")
       (map (fn [p]
              (let [f (fs/file-name p)
                    name (str/replace f #"\.duckdb$" "")
                    size (fs/size p)]
                {:name name
                 :file (str f)
                 :size_bytes size
                 :size_human (cond
                               (> size (* 1024 1024)) (format "%.1f MB" (/ size 1024.0 1024.0))
                               (> size 1024) (format "%.1f KB" (/ size 1024.0))
                               :else (format "%d B" size))})))
       (sort-by :name)))

(defn list-tables
  "List tables in a .duckdb file"
  [db-name]
  (let [db-path (str DUCK_DIR "/" db-name ".duckdb")]
    (when (fs/exists? db-path)
      (duck-query db-path
        "SELECT table_name, estimated_size, column_count
         FROM duckdb_tables()
         ORDER BY table_name"))))

(defn describe-table
  "Get column info for a table"
  [db-name table-name]
  (let [db-path (str DUCK_DIR "/" db-name ".duckdb")]
    (when (fs/exists? db-path)
      (duck-query db-path
        (format "SELECT column_name, data_type, is_nullable
                 FROM information_schema.columns
                 WHERE table_name = '%s'
                 ORDER BY ordinal_position"
                (str/replace table-name "'" "''"))))))

(defn query-table
  "Query rows from a table with pagination"
  [db-name table-name {:keys [limit offset] :or {limit PAGE_SIZE offset 0}}]
  (let [db-path (str DUCK_DIR "/" db-name ".duckdb")]
    (when (fs/exists? db-path)
      (duck-query db-path
        (format "SELECT * FROM \"%s\" LIMIT %d OFFSET %d"
                (str/replace table-name "\"" "\"\"")
                limit offset)))))

;; ══════════════════════════════════════════════════════════════════════
;; HTML Generation (trit: PLUS)
;; ══════════════════════════════════════════════════════════════════════

(defn breadcrumb-html
  "Generate breadcrumb navigation bar"
  [segments]
  (let [paths (reductions (fn [acc seg] (str acc "/" seg)) "" segments)
        pairs (map vector segments (rest paths))]
    (str "<nav class='breadcrumb'>"
         "<a href='/'>basin.duck</a>"
         (apply str (map (fn [[seg path]]
                           (str " / <a href='" path "'>" seg "</a>"))
                         pairs))
         "</nav>")))

(defn page-html
  "Wrap content in HTML page"
  [title breadcrumbs content]
  (format "<!DOCTYPE html>
<html><head>
<meta charset='utf-8'>
<title>%s - basin.duck</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; margin: 2em; background: #0d1117; color: #c9d1d9; }
  a { color: #58a6ff; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .breadcrumb { padding: 0.5em 0; border-bottom: 1px solid #30363d; margin-bottom: 1em; }
  table { border-collapse: collapse; width: 100%%; margin: 1em 0; }
  th, td { text-align: left; padding: 0.4em 0.8em; border: 1px solid #30363d; }
  th { background: #161b22; }
  tr:hover { background: #161b22; }
  .size { color: #8b949e; font-size: 0.9em; }
  .trit { display: inline-block; width: 0.6em; height: 0.6em; border-radius: 50%%; margin-right: 0.3em; }
  .trit-plus { background: #3fb950; }
  .trit-ergodic { background: #58a6ff; }
  .trit-minus { background: #f85149; }
  .gf3 { font-size: 0.8em; color: #8b949e; margin-top: 2em; }
  code { background: #161b22; padding: 0.1em 0.3em; border-radius: 3px; }
</style>
</head><body>
%s
<h1>%s</h1>
%s
<div class='gf3'>
  <span class='trit trit-plus'></span>PLUS(+1)
  <span class='trit trit-ergodic'></span>ERGODIC(0)
  <span class='trit trit-minus'></span>MINUS(-1)
  &nbsp;|&nbsp; GF(3) conserved: &Sigma; trit = 0 mod 3
</div>
</body></html>"
          title
          (breadcrumb-html breadcrumbs)
          title
          content))

(defn databases-html
  "Render list of databases as HTML"
  [databases]
  (str "<table>"
       "<tr><th>Database</th><th>Size</th></tr>"
       (apply str (map (fn [{:keys [name size_human]}]
                         (format "<tr><td><a href='/%s'>%s</a></td><td class='size'>%s</td></tr>"
                                 name name size_human))
                       databases))
       "</table>"
       (format "<p class='size'>%d databases, %s total</p>"
               (count databases)
               (let [total (reduce + (map :size_bytes databases))]
                 (format "%.1f MB" (/ total 1024.0 1024.0))))))

(defn tables-html
  "Render table listing as HTML"
  [db-name tables]
  (if (:ok tables)
    (str "<table>"
         "<tr><th>Table</th><th>~Rows</th><th>Columns</th></tr>"
         (apply str (map (fn [{:keys [table_name estimated_size column_count]}]
                           (format "<tr><td><a href='/%s/%s'>%s</a></td><td class='size'>%s</td><td class='size'>%d</td></tr>"
                                   db-name table_name table_name
                                   (or estimated_size "?")
                                   (or column_count 0)))
                         (:data tables)))
         "</table>")
    (format "<p style='color:#f85149'>Error: %s</p>" (:error tables))))

(defn rows-html
  "Render query result rows as HTML table"
  [data]
  (if (and (:ok data) (seq (:data data)))
    (let [rows (:data data)
          cols (keys (first rows))]
      (str "<table>"
           "<tr>" (apply str (map #(format "<th>%s</th>" (name %)) cols)) "</tr>"
           (apply str (map (fn [row]
                             (str "<tr>"
                                  (apply str (map (fn [c]
                                                    (format "<td>%s</td>"
                                                            (let [v (get row c)]
                                                              (if (nil? v) "<em>null</em>" (str v)))))
                                                  cols))
                                  "</tr>"))
                           rows))
           "</table>"))
    (if (:ok data)
      "<p>No rows.</p>"
      (format "<p style='color:#f85149'>Error: %s</p>" (:error data)))))

;; ══════════════════════════════════════════════════════════════════════
;; HTTP Router (trit: ERGODIC)
;; ══════════════════════════════════════════════════════════════════════

(defn parse-query-params
  "Parse query string into map"
  [qs]
  (when qs
    (->> (str/split qs #"&")
         (map #(str/split % #"=" 2))
         (filter #(= 2 (count %)))
         (map (fn [[k v]] [(keyword k) v]))
         (into {}))))

(defn handler [req]
  (let [path (str/replace (:uri req) #"^/|/$" "")
        segments (if (empty? path) [] (str/split path #"/"))
        params (parse-query-params (:query-string req))]
    {:status 200
     :headers {"Content-Type" "text/html; charset=utf-8"
               "X-GF3-Trit" "0"
               "X-YARD-Version" "0.1.0"}
     :body
     (case (count segments)
       ;; Root: list databases
       0 (page-html "Databases" []
                    (databases-html (list-databases)))
       ;; 1 segment: list tables in database
       1 (let [db (first segments)
               tables (list-tables db)]
           (if tables
             (page-html db [db] (tables-html db tables))
             (page-html "Not Found" [db]
                        (format "<p>Database <code>%s.duckdb</code> not found in <code>%s</code></p>"
                                db DUCK_DIR))))
       ;; 2 segments: query table
       2 (let [[db table] segments
               limit (or (some-> (:limit params) parse-long) PAGE_SIZE)
               offset (or (some-> (:offset params) parse-long) 0)
               schema (describe-table db table)
               data (query-table db table {:limit limit :offset offset})]
           (page-html (str db "/" table) [db table]
                      (str (when (:ok schema)
                             (str "<h2>Schema</h2>"
                                  "<table><tr><th>Column</th><th>Type</th><th>Nullable</th></tr>"
                                  (apply str (map (fn [{:keys [column_name data_type is_nullable]}]
                                                    (format "<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>"
                                                            column_name data_type is_nullable))
                                                  (:data schema)))
                                  "</table>"))
                           "<h2>Data</h2>"
                           (rows-html data)
                           (format "<p class='size'>Showing %d-%d (limit %d) &nbsp; "
                                   offset (+ offset limit) limit)
                           (when (> offset 0)
                             (format "<a href='/%s/%s?limit=%d&offset=%d'>&laquo; prev</a> "
                                     db table limit (max 0 (- offset limit))))
                           (format "<a href='/%s/%s?limit=%d&offset=%d'>next &raquo;</a></p>"
                                   db table limit (+ offset limit)))))
       ;; 3+ segments: deep path (future: column-level drill)
       (page-html "Deep Path" segments
                  (format "<p>Deep breadcrumb: <code>%s</code></p><p>Not yet implemented. Try 2-level paths.</p>"
                          (str/join "/" segments))))}))

;; ══════════════════════════════════════════════════════════════════════
;; CLI
;; ══════════════════════════════════════════════════════════════════════

(defn -main [& args]
  (let [cmd (first args)]
    (case cmd
      "serve"
      (let [port (or (some-> (second args)
                             (when (= "--port" (second args)) (nth args 2 nil))
                             parse-long)
                     PORT)]
        (println (format "YARD .duck server starting on :%d" port))
        (println (format "  DUCK_DIR: %s" DUCK_DIR))
        (println (format "  Databases: %d" (count (list-databases))))
        (println (format "  GF(3): server=0(ERGODIC) query=-1(MINUS) html=+1(PLUS) sum=0"))
        (println)
        (println "  Navigate to http://basin.duck/ (requires Pi-hole *.duck -> 127.0.0.1)")
        (println "  Or directly: http://localhost:" port "/")
        (http/run-server handler {:port port})
        @(promise))

      "list"
      (doseq [{:keys [name size_human]} (list-databases)]
        (println (format "  %-40s %s" name size_human)))

      "query"
      (let [[_ db sql] args
            result (duck-query (str DUCK_DIR "/" db ".duckdb") sql)]
        (if (:ok result)
          (println (json/encode (:data result) {:pretty true}))
          (binding [*out* *err*] (println (:error result)))))

      "schema"
      (let [db (second args)
            result (list-tables db)]
        (if (:ok result)
          (doseq [{:keys [table_name estimated_size column_count]} (:data result)]
            (println (format "  %-40s ~%s rows, %d cols" table_name (or estimated_size "?") (or column_count 0))))
          (println "Error:" (:error result))))

      ;; default
      (do
        (println "yard-duck.bb — YARD documentation for .duck domains")
        (println)
        (println "Usage:")
        (println "  bb yard-duck.bb serve [--port N]    Start HTTP server")
        (println "  bb yard-duck.bb list                List .duckdb files")
        (println "  bb yard-duck.bb query <db> <sql>    Query database")
        (println "  bb yard-duck.bb schema <db>         Show table listing")))))

(apply -main *command-line-args*)
