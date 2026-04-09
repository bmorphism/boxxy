;; DuckDB-backed seL4 IPC Monitoring & GF(3) Conservation Ledger
;; Persistent storage for IPC messages, capabilities, faults, and GF(3) sessions
;;
;; Uses DuckDB via JDBC for:
;;   - High-performance columnar analytics on IPC traffic
;;   - GF(3) trit conservation auditing across sessions
;;   - Parquet export for offline analysis

(ns sel4.duckdb-ipc
  "DuckDB storage for seL4 IPC monitoring and GF(3) enforcement"
  (:require [clojure.java.jdbc :as jdbc]
            [clojure.string :as str]
            [sel4.balance :as bal]
            [sel4.ipc :as ipc]))

;; ============================================================================
;; GF(3) Trit Constants (mirrored from sel4.balance)
;; ============================================================================

(def TRIT_MINUS -1)  ;; Validation/verification operations
(def TRIT_ZERO   0)  ;; Coordination/message operations
(def TRIT_PLUS   1)  ;; Generation/creation operations

;; ============================================================================
;; Schema Definitions
;; ============================================================================

(def schema-ipc-messages
  "CREATE TABLE IF NOT EXISTS ipc_messages (
     timestamp    BIGINT NOT NULL,
     direction    VARCHAR NOT NULL,
     endpoint     VARCHAR NOT NULL,
     operation    VARCHAR NOT NULL,
     badge        BIGINT DEFAULT 0,
     args_json    VARCHAR DEFAULT '[]',
     trit         TINYINT NOT NULL CHECK (trit IN (-1, 0, 1)),
     session_id   VARCHAR
   )")

(def schema-capabilities
  "CREATE TABLE IF NOT EXISTS capabilities (
     id           VARCHAR PRIMARY KEY,
     name         VARCHAR NOT NULL,
     endpoint     VARCHAR NOT NULL,
     rights       BIGINT DEFAULT 255,
     trit         TINYINT NOT NULL CHECK (trit IN (-1, 0, 1)),
     created_at   BIGINT NOT NULL,
     expires_at   BIGINT NOT NULL,
     revoked      BOOLEAN DEFAULT FALSE
   )")

(def schema-faults
  "CREATE TABLE IF NOT EXISTS faults (
     timestamp    BIGINT NOT NULL,
     type         VARCHAR NOT NULL,
     address      BIGINT DEFAULT 0,
     fault_ip     BIGINT DEFAULT 0,
     badge        BIGINT DEFAULT 0,
     action       VARCHAR NOT NULL,
     reason       VARCHAR
   )")

(def schema-gf3-sessions
  "CREATE TABLE IF NOT EXISTS gf3_sessions (
     session_id     VARCHAR PRIMARY KEY,
     start_time     BIGINT NOT NULL,
     end_time       BIGINT,
     message_count  INTEGER DEFAULT 0,
     trit_sum       INTEGER DEFAULT 0,
     balanced       BOOLEAN DEFAULT TRUE
   )")

(def schema-gf3-violations
  "CREATE TABLE IF NOT EXISTS gf3_violations (
     timestamp      BIGINT NOT NULL,
     session_id     VARCHAR,
     message_id     BIGINT,
     expected_trit  TINYINT NOT NULL,
     actual_trit    TINYINT NOT NULL,
     endpoint       VARCHAR NOT NULL,
     operation      VARCHAR NOT NULL
   )")

(def all-schemas
  [schema-ipc-messages
   schema-capabilities
   schema-faults
   schema-gf3-sessions
   schema-gf3-violations])

;; ============================================================================
;; Database Connection
;; ============================================================================

(defn db-spec
  "Build JDBC connection spec for DuckDB at given path.
   Use :memory: for in-memory database."
  [db-path]
  {:classname   "org.duckdb.DuckDBDriver"
   :subprotocol "duckdb"
   :subname     db-path})

(defn init-db
  "Create/open a DuckDB database and create all tables if not exist.
   Returns the db-spec for use with other functions."
  [db-path]
  (let [spec (db-spec db-path)]
    (doseq [ddl all-schemas]
      (jdbc/execute! spec ddl))
    spec))

;; ============================================================================
;; IPC Message Recording
;; ============================================================================

(defn record-ipc!
  "Insert an IPC message row and enforce GF(3) via bal/check-or-warn.

   Parameters:
     db         - db-spec from init-db
     direction  - :send or :recv
     endpoint   - seL4 endpoint string
     operation  - operation keyword
     badge      - sender badge (long)
     args       - argument vector (serialized to JSON string)
     session-id - optional GF(3) monitoring session ID

   Also calls bal/check-or-warn for GF(3) enforcement."
  [db {:keys [direction endpoint operation badge args session-id]
       :or   {badge 0 args [] session-id nil}}]
  (let [now       (System/currentTimeMillis)
        op-name   (if (keyword? operation) (name operation) (str operation))
        trit      (bal/operation-trit (if (keyword? operation) operation
                                         (keyword operation)))
        args-json (str "[" (str/join "," (map str args)) "]")
        msg       (ipc/->Message endpoint
                                  (if (keyword? operation) operation
                                      (keyword operation))
                                  0 args 0 [])]

    ;; GF(3) enforcement — check-or-warn may throw if enforcement is enabled
    (bal/check-or-warn msg)

    ;; Insert into DuckDB
    (jdbc/insert! db :ipc_messages
      {:timestamp   now
       :direction   (name direction)
       :endpoint    endpoint
       :operation   op-name
       :badge       badge
       :args_json   args-json
       :trit        trit
       :session_id  session-id})

    ;; Update session counters if session is active
    (when session-id
      (jdbc/execute! db
        ["UPDATE gf3_sessions
          SET message_count = message_count + 1,
              trit_sum = trit_sum + ?,
              balanced = ((trit_sum + ?) % 3 = 0)
          WHERE session_id = ? AND end_time IS NULL"
         trit trit session-id]))

    {:status :recorded :timestamp now :trit trit}))

;; ============================================================================
;; Capability Recording
;; ============================================================================

(defn record-capability!
  "Insert or update a capability record.

   Parameters:
     db  - db-spec from init-db
     cap - map with :id :name :endpoint :rights :trit :created-at :expires-at
           or a sel4.capability/Capability record"
  [db cap]
  (let [now  (System/currentTimeMillis)
        id   (str (or (:id cap) (java.util.UUID/randomUUID)))
        trit (or (:trit cap) TRIT_ZERO)
        row  {:id         id
              :name       (or (:name cap) "unnamed")
              :endpoint   (or (:endpoint cap) "sel4:unbound")
              :rights     (or (:rights cap) 0xFF)
              :trit       trit
              :created_at (or (:created-at cap) (:created_at cap) now)
              :expires_at (or (:expires-at cap) (:expires_at cap) (+ now 86400000))
              :revoked    (boolean (or (:revoked cap) false))}]

    ;; Upsert: delete then insert (DuckDB doesn't have native UPSERT in all versions)
    (jdbc/execute! db ["DELETE FROM capabilities WHERE id = ?" id])
    (jdbc/insert! db :capabilities row)
    {:status :recorded :id id :trit trit}))

;; ============================================================================
;; Fault Recording
;; ============================================================================

(defn record-fault!
  "Insert a fault record.

   Parameters:
     db    - db-spec from init-db
     fault - map with :type :address :fault-ip :badge :action :reason"
  [db {:keys [type address fault-ip badge action reason]
       :or   {address 0 fault-ip 0 badge 0 reason nil}}]
  (let [now (System/currentTimeMillis)
        row {:timestamp now
             :type      (if (keyword? type) (name type) (str type))
             :address   address
             :fault_ip  fault-ip
             :badge     badge
             :action    (if (keyword? action) (name action) (str action))
             :reason    reason}]

    (jdbc/insert! db :faults row)
    {:status :recorded :timestamp now :type type}))

;; ============================================================================
;; GF(3) Session Management
;; ============================================================================

(defn start-session!
  "Begin a GF(3) monitoring session.
   Returns session-id for use with record-ipc! and other functions."
  [db & {:keys [session-id] :or {session-id nil}}]
  (let [sid (or session-id (str (java.util.UUID/randomUUID)))
        now (System/currentTimeMillis)]

    (jdbc/insert! db :gf3_sessions
      {:session_id    sid
       :start_time    now
       :end_time      nil
       :message_count 0
       :trit_sum      0
       :balanced      true})

    {:status :started :session-id sid :start-time now}))

(defn end-session!
  "Close a GF(3) monitoring session. Computes final balance."
  [db session-id]
  (let [now (System/currentTimeMillis)]

    (jdbc/execute! db
      ["UPDATE gf3_sessions
        SET end_time = ?,
            balanced = (trit_sum % 3 = 0)
        WHERE session_id = ? AND end_time IS NULL"
       now session-id])

    (let [session (first (jdbc/query db
                           ["SELECT * FROM gf3_sessions WHERE session_id = ?"
                            session-id]))]
      {:status     :ended
       :session-id session-id
       :end-time   now
       :balanced   (:balanced session)
       :trit-sum   (:trit_sum session)
       :messages   (:message_count session)})))

;; ============================================================================
;; GF(3) Violation Recording
;; ============================================================================

(defn record-violation!
  "Log a GF(3) trit violation.

   Parameters:
     db            - db-spec from init-db
     session-id    - monitoring session (may be nil)
     message-id    - originating message timestamp or ID
     expected-trit - what the trit should have been
     actual-trit   - what the trit actually was
     endpoint      - the offending endpoint
     operation     - the offending operation"
  [db {:keys [session-id message-id expected-trit actual-trit endpoint operation]}]
  (let [now (System/currentTimeMillis)]

    (jdbc/insert! db :gf3_violations
      {:timestamp     now
       :session_id    session-id
       :message_id    (or message-id 0)
       :expected_trit expected-trit
       :actual_trit   actual-trit
       :endpoint      (or endpoint "unknown")
       :operation     (if (keyword? operation) (name operation) (str operation))})

    {:status :violation-recorded :timestamp now}))

;; ============================================================================
;; Query Functions
;; ============================================================================

(defn query-balance
  "SELECT session stats with trit distribution.
   Returns per-session breakdown of MINUS/ZERO/PLUS message counts."
  [db & {:keys [session-id] :or {session-id nil}}]
  (let [base-sql
        "SELECT
           s.session_id,
           s.start_time,
           s.end_time,
           s.message_count,
           s.trit_sum,
           s.balanced,
           COUNT(CASE WHEN m.trit = -1 THEN 1 END) AS minus_count,
           COUNT(CASE WHEN m.trit = 0  THEN 1 END) AS zero_count,
           COUNT(CASE WHEN m.trit = 1  THEN 1 END) AS plus_count
         FROM gf3_sessions s
         LEFT JOIN ipc_messages m ON s.session_id = m.session_id"
        where-clause (if session-id
                       " WHERE s.session_id = ?"
                       "")
        group-clause " GROUP BY s.session_id, s.start_time, s.end_time,
                                s.message_count, s.trit_sum, s.balanced
                       ORDER BY s.start_time DESC"
        sql (str base-sql where-clause group-clause)]

    (if session-id
      (jdbc/query db [sql session-id])
      (jdbc/query db [sql]))))

(defn query-violations
  "Find all GF(3) violations in a time range.

   Parameters:
     db    - db-spec
     from  - start timestamp (millis)
     to    - end timestamp (millis)"
  [db from to]
  (jdbc/query db
    ["SELECT * FROM gf3_violations
      WHERE timestamp >= ? AND timestamp <= ?
      ORDER BY timestamp DESC"
     from to]))

(defn query-hot-endpoints
  "Top endpoints by message count.

   Parameters:
     db    - db-spec
     limit - max results (default 10)"
  [db & {:keys [limit] :or {limit 10}}]
  (jdbc/query db
    ["SELECT
        endpoint,
        COUNT(*) AS msg_count,
        SUM(CASE WHEN trit = -1 THEN 1 ELSE 0 END) AS minus_msgs,
        SUM(CASE WHEN trit = 0  THEN 1 ELSE 0 END) AS zero_msgs,
        SUM(CASE WHEN trit = 1  THEN 1 ELSE 0 END) AS plus_msgs,
        SUM(trit) AS trit_sum,
        (SUM(trit) % 3 = 0) AS balanced
      FROM ipc_messages
      GROUP BY endpoint
      ORDER BY msg_count DESC
      LIMIT ?"
     limit]))

(defn query-capability-tree
  "Capabilities grouped by trit class.
   Returns three groups: validators (MINUS), coordinators (ZERO), generators (PLUS)."
  [db]
  (let [caps (jdbc/query db
               ["SELECT * FROM capabilities
                 WHERE revoked = FALSE
                 ORDER BY trit, created_at DESC"])]
    {:validators   (filter #(= (:trit %) TRIT_MINUS) caps)
     :coordinators (filter #(= (:trit %) TRIT_ZERO)  caps)
     :generators   (filter #(= (:trit %) TRIT_PLUS)  caps)}))

;; ============================================================================
;; Parquet Export
;; ============================================================================

(defn export-session-parquet
  "Export a session's IPC messages to a Parquet file.
   DuckDB has native Parquet support via COPY ... TO ... (FORMAT PARQUET).

   Parameters:
     db          - db-spec
     session-id  - session to export
     output-path - path for the .parquet file"
  [db session-id output-path]
  (jdbc/execute! db
    [(str "COPY (
            SELECT m.*, s.trit_sum AS session_trit_sum, s.balanced AS session_balanced
            FROM ipc_messages m
            JOIN gf3_sessions s ON m.session_id = s.session_id
            WHERE m.session_id = '" session-id "'
            ORDER BY m.timestamp
          ) TO '" output-path "' (FORMAT PARQUET)")])
  {:status :exported :session-id session-id :path output-path})

;; ============================================================================
;; Demo Queries
;; ============================================================================

(defn demo-queries
  "Print example SQL queries users can run against the DuckDB database."
  []
  (let [queries
        [["GF(3) balance per session"
          "SELECT session_id, message_count, trit_sum,
                  trit_sum % 3 AS residue, balanced
           FROM gf3_sessions
           ORDER BY start_time DESC;"]

         ["Trit distribution across all messages"
          "SELECT trit, COUNT(*) AS cnt,
                  ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) AS pct
           FROM ipc_messages
           GROUP BY trit
           ORDER BY trit;"]

         ["Hot endpoints (top 10 by traffic)"
          "SELECT endpoint, COUNT(*) AS msgs,
                  SUM(trit) AS trit_sum,
                  (SUM(trit) % 3 = 0) AS balanced
           FROM ipc_messages
           GROUP BY endpoint
           ORDER BY msgs DESC
           LIMIT 10;"]

         ["GF(3) violations with session context"
          "SELECT v.timestamp, v.endpoint, v.operation,
                  v.expected_trit, v.actual_trit,
                  s.trit_sum AS session_trit_at_violation
           FROM gf3_violations v
           LEFT JOIN gf3_sessions s ON v.session_id = s.session_id
           ORDER BY v.timestamp DESC;"]

         ["Capability tree by trit class"
          "SELECT
             CASE trit WHEN -1 THEN 'Validator'
                       WHEN  0 THEN 'Coordinator'
                       WHEN  1 THEN 'Generator' END AS role,
             COUNT(*) AS cap_count,
             COUNT(CASE WHEN revoked THEN 1 END) AS revoked_count
           FROM capabilities
           GROUP BY trit
           ORDER BY trit;"]

         ["Faults by type (last 24h)"
          "SELECT type, COUNT(*) AS cnt, action
           FROM faults
           WHERE timestamp > (epoch_ms(now()) - 86400000)
           GROUP BY type, action
           ORDER BY cnt DESC;"]

         ["Export session to Parquet"
          "COPY (
             SELECT * FROM ipc_messages
             WHERE session_id = '<YOUR_SESSION_ID>'
           ) TO '/tmp/session_export.parquet' (FORMAT PARQUET);"]

         ["Unbalanced sessions (conservation violations)"
          "SELECT session_id, message_count, trit_sum,
                  trit_sum % 3 AS residue
           FROM gf3_sessions
           WHERE balanced = FALSE AND end_time IS NOT NULL
           ORDER BY end_time DESC;"]]]

    (println "\n=== DuckDB seL4 IPC Monitor — Example Queries ===\n")
    (doseq [[title sql] queries]
      (println (str "-- " title))
      (println sql)
      (println))))
