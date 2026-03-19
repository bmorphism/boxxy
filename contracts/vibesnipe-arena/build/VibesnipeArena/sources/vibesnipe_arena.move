module vibesnipe_arena::vibesnipe_arena {
    use std::signer;
    use std::vector;
    use std::string::String;
    use aptos_framework::event;
    use aptos_framework::timestamp;
    use aptos_std::simple_map::{Self, SimpleMap};
    use aptos_std::table::{Self, Table};
    use vibesnipe_arena::bounty_pool;

    // ===================== Error codes =====================

    const E_NOT_ADMIN: u64 = 1;
    const E_MARKETPLACE_EXISTS: u64 = 2;
    const E_MARKETPLACE_NOT_FOUND: u64 = 3;
    const E_RUNTIME_EXISTS: u64 = 4;
    const E_RUNTIME_NOT_FOUND: u64 = 5;
    const E_INVALID_TRIT: u64 = 6;
    const E_GF3_IMBALANCED: u64 = 7;
    const E_EXPLOIT_NOT_FOUND: u64 = 8;
    const E_ALREADY_VOTED: u64 = 9;
    const E_NOT_RUNTIME_OPERATOR: u64 = 10;
    const E_INVALID_SEVERITY: u64 = 11;
    const E_ALREADY_VERIFIED: u64 = 12;
    const E_NOT_VERIFIED: u64 = 13;
    const E_ALREADY_CLAIMED: u64 = 14;
    const E_ROUND_NOT_ACTIVE: u64 = 15;
    const E_EMPTY_PROOF: u64 = 16;

    // ===================== Exploit class constants =====================
    // Mirrors Go ExploitClass enum in internal/exploit_arena/marketplace.go

    const CLASS_TIMING_ATTACK: u8 = 0;
    const CLASS_MEMORY_SIDECHANNEL: u8 = 1;
    const CLASS_CONTROLFLOW: u8 = 2;
    const CLASS_DATAFLOW: u8 = 3;
    const CLASS_QUANTUM: u8 = 4;
    const CLASS_CONSENSUS_BREAK: u8 = 5;
    const CLASS_MEMBRANE_BREACH: u8 = 6;
    const CLASS_REVOCATION_BYPASS: u8 = 7;

    // ===================== GF(3) trit constants =====================
    // Encoded as u8 to avoid signed arithmetic in Move.
    // MINUS=0 (represents -1), ERGODIC=1 (represents 0), PLUS=2 (represents +1)

    const TRIT_MINUS: u8 = 0;
    const TRIT_ERGODIC: u8 = 1;
    const TRIT_PLUS: u8 = 2;

    // ===================== Structs =====================

    struct Runtime has store, copy, drop {
        id: String,
        version: String,
        gf3_trit: u8,
        vuln_count: u64,
        patch_count: u64,
        operator: address,
    }

    struct ExploitEntry has store, copy, drop {
        id: u64,
        target_runtime: String,
        exploit_class: u8,
        severity: u8,
        proof_hash: vector<u8>,
        submitter: address,
        timestamp: u64,
        // Triadic consensus votes: index 0=MINUS, 1=ERGODIC, 2=PLUS
        votes: vector<bool>,
        voted: vector<bool>,
        verified: bool,
        reward_claimed: bool,
    }

    struct Marketplace has key {
        admin: address,
        runtimes: SimpleMap<String, Runtime>,
        exploits: Table<u64, ExploitEntry>,
        exploit_count: u64,
        round: u64,
        round_active: bool,
        trit_counts: vector<u64>,
        reward_per_severity: u64,
        pool_addr: address,
    }

    // ===================== Events =====================

    #[event]
    struct RuntimeRegistered has drop, store {
        id: String,
        operator: address,
        gf3_trit: u8,
    }

    #[event]
    struct ExploitSubmitted has drop, store {
        exploit_id: u64,
        target_runtime: String,
        exploit_class: u8,
        severity: u8,
        submitter: address,
    }

    #[event]
    struct ExploitVoted has drop, store {
        exploit_id: u64,
        voter_trit: u8,
        vote: bool,
    }

    #[event]
    struct ExploitVerified has drop, store {
        exploit_id: u64,
        severity: u8,
    }

    #[event]
    struct RoundStarted has drop, store {
        round: u64,
    }

    #[event]
    struct RewardClaimed has drop, store {
        exploit_id: u64,
        submitter: address,
        amount: u64,
    }

    // ===================== Init =====================

    public entry fun initialize(admin: &signer, reward_per_severity: u64, pool_addr: address) {
        let admin_addr = signer::address_of(admin);
        assert!(!exists<Marketplace>(admin_addr), E_MARKETPLACE_EXISTS);

        move_to(admin, Marketplace {
            admin: admin_addr,
            runtimes: simple_map::new(),
            exploits: table::new(),
            exploit_count: 0,
            round: 0,
            round_active: false,
            trit_counts: vector[0u64, 0u64, 0u64],
            reward_per_severity,
            pool_addr,
        });
    }

    // ===================== Runtime registration =====================

    public entry fun register_runtime(
        operator: &signer,
        marketplace_addr: address,
        id: String,
        version: String,
        gf3_trit: u8,
    ) acquires Marketplace {
        assert!(gf3_trit <= 2, E_INVALID_TRIT);
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(!simple_map::contains_key(&mp.runtimes, &id), E_RUNTIME_EXISTS);

        let op_addr = signer::address_of(operator);
        let runtime = Runtime {
            id,
            version,
            gf3_trit,
            vuln_count: 0,
            patch_count: 0,
            operator: op_addr,
        };

        simple_map::add(&mut mp.runtimes, id, runtime);

        let counts = &mut mp.trit_counts;
        let current = *vector::borrow(counts, (gf3_trit as u64));
        *vector::borrow_mut(counts, (gf3_trit as u64)) = current + 1;

        event::emit(RuntimeRegistered {
            id,
            operator: op_addr,
            gf3_trit,
        });
    }

    // ===================== Exploit submission =====================

    public entry fun submit_exploit(
        submitter: &signer,
        marketplace_addr: address,
        target_runtime: String,
        exploit_class: u8,
        severity: u8,
        proof_hash: vector<u8>,
    ) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);
        assert!(severity >= 1 && severity <= 10, E_INVALID_SEVERITY);
        assert!(vector::length(&proof_hash) > 0, E_EMPTY_PROOF);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(mp.round_active, E_ROUND_NOT_ACTIVE);
        assert!(simple_map::contains_key(&mp.runtimes, &target_runtime), E_RUNTIME_NOT_FOUND);

        let exploit_id = mp.exploit_count;
        mp.exploit_count = exploit_id + 1;

        let sub_addr = signer::address_of(submitter);
        let now = timestamp::now_seconds();

        let entry = ExploitEntry {
            id: exploit_id,
            target_runtime,
            exploit_class,
            severity,
            proof_hash,
            submitter: sub_addr,
            timestamp: now,
            votes: vector[false, false, false],
            voted: vector[false, false, false],
            verified: false,
            reward_claimed: false,
        };

        table::add(&mut mp.exploits, exploit_id, entry);

        // Increment vuln count on target runtime
        let rt = simple_map::borrow_mut(&mut mp.runtimes, &target_runtime);
        rt.vuln_count = rt.vuln_count + 1;

        event::emit(ExploitSubmitted {
            exploit_id,
            target_runtime,
            exploit_class,
            severity,
            submitter: sub_addr,
        });
    }

    // ===================== Triadic consensus voting =====================

    public entry fun vote_exploit(
        voter: &signer,
        marketplace_addr: address,
        exploit_id: u64,
        runtime_id: String,
        vote: bool,
    ) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(table::contains(&mp.exploits, exploit_id), E_EXPLOIT_NOT_FOUND);

        let rt = simple_map::borrow(&mp.runtimes, &runtime_id);
        assert!(rt.operator == signer::address_of(voter), E_NOT_RUNTIME_OPERATOR);

        let trit = rt.gf3_trit;
        let entry = table::borrow_mut(&mut mp.exploits, exploit_id);
        assert!(!entry.verified, E_ALREADY_VERIFIED);

        let trit_idx = (trit as u64);
        assert!(!*vector::borrow(&entry.voted, trit_idx), E_ALREADY_VOTED);

        *vector::borrow_mut(&mut entry.voted, trit_idx) = true;
        *vector::borrow_mut(&mut entry.votes, trit_idx) = vote;

        event::emit(ExploitVoted {
            exploit_id,
            voter_trit: trit,
            vote,
        });

        // Check if all three trits have voted and all approved
        let all_voted = *vector::borrow(&entry.voted, 0)
            && *vector::borrow(&entry.voted, 1)
            && *vector::borrow(&entry.voted, 2);

        if (all_voted) {
            let all_approved = *vector::borrow(&entry.votes, 0)
                && *vector::borrow(&entry.votes, 1)
                && *vector::borrow(&entry.votes, 2);

            if (all_approved) {
                entry.verified = true;
                event::emit(ExploitVerified {
                    exploit_id,
                    severity: entry.severity,
                });
            };
        };
    }

    // ===================== Round management =====================

    public entry fun start_round(
        admin: &signer,
        marketplace_addr: address,
    ) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(signer::address_of(admin) == mp.admin, E_NOT_ADMIN);
        assert!(verify_gf3_balance_internal(mp), E_GF3_IMBALANCED);

        mp.round = mp.round + 1;
        mp.round_active = true;

        event::emit(RoundStarted { round: mp.round });
    }

    public entry fun end_round(
        admin: &signer,
        marketplace_addr: address,
    ) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(signer::address_of(admin) == mp.admin, E_NOT_ADMIN);

        mp.round_active = false;
    }

    // ===================== Reward claiming =====================

    public entry fun claim_reward(
        submitter: &signer,
        marketplace_addr: address,
        exploit_id: u64,
    ) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);

        let mp = borrow_global_mut<Marketplace>(marketplace_addr);
        assert!(table::contains(&mp.exploits, exploit_id), E_EXPLOIT_NOT_FOUND);

        let entry = table::borrow_mut(&mut mp.exploits, exploit_id);
        let sub_addr = signer::address_of(submitter);
        assert!(entry.submitter == sub_addr, E_NOT_RUNTIME_OPERATOR);
        assert!(entry.verified, E_NOT_VERIFIED);
        assert!(!entry.reward_claimed, E_ALREADY_CLAIMED);

        entry.reward_claimed = true;
        let gross_amount = (entry.severity as u64) * mp.reward_per_severity;
        let pool_addr = mp.pool_addr;

        let net_amount = bounty_pool::pay_bounty(pool_addr, sub_addr, exploit_id, gross_amount);

        event::emit(RewardClaimed {
            exploit_id,
            submitter: sub_addr,
            amount: net_amount,
        });
    }

    // ===================== View functions =====================

    #[view]
    public fun verify_gf3_balance(marketplace_addr: address): bool acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);
        let mp = borrow_global<Marketplace>(marketplace_addr);
        verify_gf3_balance_internal(mp)
    }

    #[view]
    public fun get_exploit(
        marketplace_addr: address,
        exploit_id: u64,
    ): (String, u8, u8, address, bool) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);
        let mp = borrow_global<Marketplace>(marketplace_addr);
        assert!(table::contains(&mp.exploits, exploit_id), E_EXPLOIT_NOT_FOUND);

        let entry = table::borrow(&mp.exploits, exploit_id);
        (entry.target_runtime, entry.exploit_class, entry.severity, entry.submitter, entry.verified)
    }

    #[view]
    public fun get_runtime(
        marketplace_addr: address,
        id: String,
    ): (String, u8, u64, u64, address) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);
        let mp = borrow_global<Marketplace>(marketplace_addr);
        assert!(simple_map::contains_key(&mp.runtimes, &id), E_RUNTIME_NOT_FOUND);

        let rt = simple_map::borrow(&mp.runtimes, &id);
        (rt.version, rt.gf3_trit, rt.vuln_count, rt.patch_count, rt.operator)
    }

    #[view]
    public fun get_arena_stats(marketplace_addr: address): (u64, u64, u64, bool, bool) acquires Marketplace {
        assert!(exists<Marketplace>(marketplace_addr), E_MARKETPLACE_NOT_FOUND);
        let mp = borrow_global<Marketplace>(marketplace_addr);

        let runtime_count = simple_map::length(&mp.runtimes);
        let balanced = verify_gf3_balance_internal(mp);

        (runtime_count, mp.exploit_count, mp.round, balanced, mp.round_active)
    }

    // ===================== Internal helpers =====================

    fun verify_gf3_balance_internal(mp: &Marketplace): bool {
        // GF(3) balance: trit values are MINUS=0(-1), ERGODIC=1(0), PLUS=2(+1)
        // Real values: minus_count * (-1) + ergodic_count * 0 + plus_count * 1
        // Balance requires: plus_count - minus_count ≡ 0 (mod 3)
        let minus_count = *vector::borrow(&mp.trit_counts, 0);
        let plus_count = *vector::borrow(&mp.trit_counts, 2);

        // Handle underflow-safe modular arithmetic
        if (plus_count >= minus_count) {
            (plus_count - minus_count) % 3 == 0
        } else {
            (minus_count - plus_count) % 3 == 0
        }
    }
}
