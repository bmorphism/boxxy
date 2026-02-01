#!/bin/bash
# Query the Belief Revision ACSet

DB="/tmp/catcolab_timetravel.duckdb"

case "$1" in
    timeline)
        duckdb "$DB" -c "SELECT week, repo, commits, revisions, indeterministic FROM BeliefStateTimeline ORDER BY week DESC LIMIT 20;"
        ;;
    entropy)
        duckdb "$DB" -c "SELECT repo, total_commits, indet_commits, entropy_bits FROM (SELECT repo, count(*) as total_commits, count(*) FILTER (WHERE creates_indeterminism) as indet_commits, round(-1.0 * ((count(*) FILTER (WHERE creates_indeterminism)::FLOAT / count(*)) * ln((count(*) FILTER (WHERE creates_indeterminism)::FLOAT / count(*)) + 0.0001) + (count(*) FILTER (WHERE NOT creates_indeterminism)::FLOAT / count(*)) * ln((count(*) FILTER (WHERE NOT creates_indeterminism)::FLOAT / count(*)) + 0.0001)), 3) as entropy_bits FROM CommitBeliefBridge GROUP BY repo) ORDER BY entropy_bits DESC;"
        ;;
    spheres)
        duckdb "$DB" -c "SELECT * FROM GroveSphereSystem ORDER BY belief_set, incoming_edges;"
        ;;
    walk)
        duckdb "$DB" -c "SELECT step, current_state, state_color, CASE trit WHEN -1 THEN '−' WHEN 0 THEN '○' WHEN 1 THEN '+' END as trit FROM BeliefWalkSimulation ORDER BY step;"
        ;;
    crossrepo)
        duckdb "$DB" -c "SELECT author, belief_domains, transfer_path, total_revisions FROM (SELECT author, count(DISTINCT repo) as belief_domains, string_agg(DISTINCT repo, ' → ') as transfer_path, count(*) as total_revisions FROM CommitBeliefBridge GROUP BY author HAVING count(DISTINCT repo) > 1) ORDER BY belief_domains DESC LIMIT 10;"
        ;;
    agm)
        duckdb "$DB" -c "SELECT r.name, count(*) FILTER (WHERE osp.satisfied) as passed, count(*) FILTER (WHERE NOT osp.satisfied) as failed FROM RevisionOp r JOIN OpSatisfiesPostulate osp ON r.op_id = osp.op_id GROUP BY r.op_id, r.name;"
        ;;
    summary)
        duckdb "$DB" -c "SELECT 'Objects' as category, count(*)::VARCHAR as count FROM Repo UNION ALL SELECT 'Authors', count(*)::VARCHAR FROM Author UNION ALL SELECT 'Commits', count(*)::VARCHAR FROM Commit UNION ALL SELECT 'BeliefSets', count(*)::VARCHAR FROM BeliefSet UNION ALL SELECT 'Fallbacks', count(*)::VARCHAR FROM Fallback UNION ALL SELECT 'RevisionOps', count(*)::VARCHAR FROM RevisionOp;"
        ;;
    *)
        echo "Usage: $0 {timeline|entropy|spheres|walk|crossrepo|agm|summary}"
        echo ""
        echo "Commands:"
        echo "  timeline  - Belief state evolution by week"
        echo "  entropy   - Indeterminism entropy per repo"
        echo "  spheres   - Grove sphere system with incomparability"
        echo "  walk      - 27-step chromatic walk"
        echo "  crossrepo - Cross-repo belief transfer"
        echo "  agm       - AGM postulate satisfaction"
        echo "  summary   - ACSet object counts"
        ;;
esac
