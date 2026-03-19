module vibesnipe_arena::credential_challenge {
    use std::signer;
    use std::vector;
    use std::string::String;
    use aptos_framework::event;
    use aptos_framework::timestamp;
    use aptos_std::table::{Self, Table};
    use aptos_std::simple_map::{Self, SimpleMap};

    // ===================== Error codes =====================

    const E_REGISTRY_EXISTS: u64 = 200;
    const E_REGISTRY_NOT_FOUND: u64 = 201;
    const E_CHALLENGE_NOT_FOUND: u64 = 202;
    const E_ALREADY_CLAIMED: u64 = 203;
    const E_CHALLENGE_EXPIRED: u64 = 204;
    const E_NOT_CHALLENGER: u64 = 205;
    const E_NOT_SUBJECT: u64 = 206;
    const E_ALREADY_SUBMITTED: u64 = 207;
    const E_NOT_REVIEWER: u64 = 208;
    const E_ALREADY_REVIEWED: u64 = 209;
    const E_INVALID_VERDICT: u64 = 210;
    const E_NOT_ADMIN: u64 = 211;
    const E_SUBJECT_NOT_FOUND: u64 = 212;
    const E_CHALLENGE_NOT_OPEN: u64 = 213;

    // ===================== Challenge status =====================

    const STATUS_OPEN: u8 = 0;        // issued, waiting for subject to accept
    const STATUS_ACCEPTED: u8 = 1;    // subject accepted, working on it
    const STATUS_SUBMITTED: u8 = 2;   // subject submitted PR, awaiting review
    const STATUS_UNDER_REVIEW: u8 = 3; // triadic review in progress
    const STATUS_PASSED: u8 = 4;      // PR merged, credential verified
    const STATUS_FAILED: u8 = 5;      // subject failed or timed out
    const STATUS_EXPIRED: u8 = 6;     // TTL expired before completion

    // ===================== Verdict =====================

    const VERDICT_APPROVE: u8 = 1;
    const VERDICT_REJECT: u8 = 2;

    // Default challenge TTL: 7 days in seconds
    const DEFAULT_TTL: u64 = 604800;

    // ===================== Structs =====================

    struct Challenge has store, copy, drop {
        id: u64,
        // The sniper: person issuing the challenge
        challenger: address,
        // The target: LinkedIn "expert" being vibesniped
        subject: address,
        // What they claim expertise in (e.g., "kubeflow")
        claimed_domain: String,
        // GitHub repo (e.g., "kubeflow/kubeflow")
        repo: String,
        // GitHub issue number to close
        issue_number: u64,
        // Issue title for on-chain reference
        issue_title: String,
        // SHA of the PR commit (submitted by subject)
        pr_commit_sha: vector<u8>,
        // PR number on GitHub
        pr_number: u64,
        status: u8,
        created_at: u64,
        expires_at: u64,
        // Triadic review: three reviewers vote
        reviewer_votes: vector<u8>,   // 0=not voted, 1=approve, 2=reject
        reviewer_voted: vector<bool>,
        reviewers: vector<address>,
    }

    struct SubjectProfile has store, copy, drop {
        addr: address,
        challenges_received: u64,
        challenges_passed: u64,
        challenges_failed: u64,
        domains_verified: vector<String>,
        reputation_score: u64,
    }

    struct ChallengeRegistry has key {
        admin: address,
        challenges: Table<u64, Challenge>,
        challenge_count: u64,
        subjects: SimpleMap<address, SubjectProfile>,
        // Approved reviewers (triadic consensus participants)
        reviewers: vector<address>,
        default_ttl: u64,
        total_passed: u64,
        total_failed: u64,
    }

    // ===================== Events =====================

    #[event]
    struct ChallengeIssued has drop, store {
        challenge_id: u64,
        challenger: address,
        subject: address,
        claimed_domain: String,
        repo: String,
        issue_number: u64,
    }

    #[event]
    struct ChallengeAccepted has drop, store {
        challenge_id: u64,
        subject: address,
    }

    #[event]
    struct PRSubmitted has drop, store {
        challenge_id: u64,
        pr_number: u64,
        commit_sha: vector<u8>,
    }

    #[event]
    struct ReviewVoted has drop, store {
        challenge_id: u64,
        reviewer: address,
        verdict: u8,
    }

    #[event]
    struct ChallengePassed has drop, store {
        challenge_id: u64,
        subject: address,
        claimed_domain: String,
        reputation_earned: u64,
    }

    #[event]
    struct ChallengeFailed has drop, store {
        challenge_id: u64,
        subject: address,
        claimed_domain: String,
    }

    // ===================== Init =====================

    public entry fun initialize(admin: &signer) {
        let admin_addr = signer::address_of(admin);
        assert!(!exists<ChallengeRegistry>(admin_addr), E_REGISTRY_EXISTS);

        move_to(admin, ChallengeRegistry {
            admin: admin_addr,
            challenges: table::new(),
            challenge_count: 0,
            subjects: simple_map::new(),
            reviewers: vector::empty(),
            default_ttl: DEFAULT_TTL,
            total_passed: 0,
            total_failed: 0,
        });
    }

    // ===================== Admin: add reviewers =====================

    public entry fun add_reviewer(
        admin: &signer,
        registry_addr: address,
        reviewer: address,
    ) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        assert!(signer::address_of(admin) == reg.admin, E_NOT_ADMIN);

        vector::push_back(&mut reg.reviewers, reviewer);
    }

    // ===================== Issue a vibesniping challenge =====================

    public entry fun issue_challenge(
        challenger: &signer,
        registry_addr: address,
        subject: address,
        claimed_domain: String,
        repo: String,
        issue_number: u64,
        issue_title: String,
    ) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);

        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        let challenger_addr = signer::address_of(challenger);
        let now = timestamp::now_seconds();

        let challenge_id = reg.challenge_count;
        reg.challenge_count = challenge_id + 1;

        // Pick up to 3 reviewers for triadic consensus
        let num_reviewers = vector::length(&reg.reviewers);
        let review_count = if (num_reviewers < 3) { num_reviewers } else { 3 };
        let assigned_reviewers = vector::empty<address>();
        let i = 0;
        while (i < review_count) {
            vector::push_back(&mut assigned_reviewers, *vector::borrow(&reg.reviewers, i));
            i = i + 1;
        };

        let challenge = Challenge {
            id: challenge_id,
            challenger: challenger_addr,
            subject,
            claimed_domain,
            repo,
            issue_number,
            issue_title,
            pr_commit_sha: vector::empty(),
            pr_number: 0,
            status: STATUS_OPEN,
            created_at: now,
            expires_at: now + reg.default_ttl,
            reviewer_votes: vector[0u8, 0u8, 0u8],
            reviewer_voted: vector[false, false, false],
            reviewers: assigned_reviewers,
        };

        table::add(&mut reg.challenges, challenge_id, challenge);

        // Create subject profile if new
        if (!simple_map::contains_key(&reg.subjects, &subject)) {
            simple_map::add(&mut reg.subjects, subject, SubjectProfile {
                addr: subject,
                challenges_received: 0,
                challenges_passed: 0,
                challenges_failed: 0,
                domains_verified: vector::empty(),
                reputation_score: 0,
            });
        };
        let profile = simple_map::borrow_mut(&mut reg.subjects, &subject);
        profile.challenges_received = profile.challenges_received + 1;

        event::emit(ChallengeIssued {
            challenge_id,
            challenger: challenger_addr,
            subject,
            claimed_domain,
            repo,
            issue_number,
        });
    }

    // ===================== Subject accepts the challenge =====================

    public entry fun accept_challenge(
        subject: &signer,
        registry_addr: address,
        challenge_id: u64,
    ) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        assert!(table::contains(&reg.challenges, challenge_id), E_CHALLENGE_NOT_FOUND);

        let challenge = table::borrow_mut(&mut reg.challenges, challenge_id);
        let sub_addr = signer::address_of(subject);
        assert!(challenge.subject == sub_addr, E_NOT_SUBJECT);
        assert!(challenge.status == STATUS_OPEN, E_CHALLENGE_NOT_OPEN);

        let now = timestamp::now_seconds();
        assert!(now < challenge.expires_at, E_CHALLENGE_EXPIRED);

        challenge.status = STATUS_ACCEPTED;

        event::emit(ChallengeAccepted {
            challenge_id,
            subject: sub_addr,
        });
    }

    // ===================== Subject submits PR evidence =====================

    public entry fun submit_pr(
        subject: &signer,
        registry_addr: address,
        challenge_id: u64,
        pr_number: u64,
        commit_sha: vector<u8>,
    ) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        assert!(table::contains(&reg.challenges, challenge_id), E_CHALLENGE_NOT_FOUND);

        let challenge = table::borrow_mut(&mut reg.challenges, challenge_id);
        let sub_addr = signer::address_of(subject);
        assert!(challenge.subject == sub_addr, E_NOT_SUBJECT);
        assert!(challenge.status == STATUS_ACCEPTED, E_ALREADY_SUBMITTED);

        let now = timestamp::now_seconds();
        assert!(now < challenge.expires_at, E_CHALLENGE_EXPIRED);

        challenge.pr_number = pr_number;
        challenge.pr_commit_sha = commit_sha;
        challenge.status = STATUS_SUBMITTED;

        event::emit(PRSubmitted {
            challenge_id,
            pr_number,
            commit_sha,
        });
    }

    // ===================== Triadic review (human reviewers vote) =====================

    public entry fun review_challenge(
        reviewer: &signer,
        registry_addr: address,
        challenge_id: u64,
        verdict: u8,
    ) acquires ChallengeRegistry {
        assert!(verdict == VERDICT_APPROVE || verdict == VERDICT_REJECT, E_INVALID_VERDICT);
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);

        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        assert!(table::contains(&reg.challenges, challenge_id), E_CHALLENGE_NOT_FOUND);

        let challenge = table::borrow_mut(&mut reg.challenges, challenge_id);
        assert!(challenge.status == STATUS_SUBMITTED || challenge.status == STATUS_UNDER_REVIEW,
            E_CHALLENGE_NOT_OPEN);

        let reviewer_addr = signer::address_of(reviewer);

        // Find reviewer index in assigned reviewers
        let idx = find_reviewer_index(&challenge.reviewers, reviewer_addr);
        assert!(idx < 3, E_NOT_REVIEWER);
        assert!(!*vector::borrow(&challenge.reviewer_voted, idx), E_ALREADY_REVIEWED);

        *vector::borrow_mut(&mut challenge.reviewer_voted, idx) = true;
        *vector::borrow_mut(&mut challenge.reviewer_votes, idx) = verdict;
        challenge.status = STATUS_UNDER_REVIEW;

        event::emit(ReviewVoted {
            challenge_id,
            reviewer: reviewer_addr,
            verdict,
        });

        // Check if all reviewers voted
        let all_voted = *vector::borrow(&challenge.reviewer_voted, 0)
            && *vector::borrow(&challenge.reviewer_voted, 1)
            && *vector::borrow(&challenge.reviewer_voted, 2);

        if (all_voted) {
            // Tally: majority wins
            let approvals = 0u64;
            let i = 0;
            while (i < 3) {
                if (*vector::borrow(&challenge.reviewer_votes, i) == VERDICT_APPROVE) {
                    approvals = approvals + 1;
                };
                i = i + 1;
            };

            let subject_addr = challenge.subject;
            let domain = challenge.claimed_domain;

            if (approvals >= 2) {
                challenge.status = STATUS_PASSED;
                reg.total_passed = reg.total_passed + 1;

                // Update subject profile
                let profile = simple_map::borrow_mut(&mut reg.subjects, &subject_addr);
                profile.challenges_passed = profile.challenges_passed + 1;
                // Reputation: 100 per pass, compounds
                profile.reputation_score = profile.reputation_score + 100;
                // Add verified domain if not already there
                if (!vector_contains(&profile.domains_verified, &domain)) {
                    vector::push_back(&mut profile.domains_verified, domain);
                };

                event::emit(ChallengePassed {
                    challenge_id,
                    subject: subject_addr,
                    claimed_domain: domain,
                    reputation_earned: 100,
                });
            } else {
                challenge.status = STATUS_FAILED;
                reg.total_failed = reg.total_failed + 1;

                let profile = simple_map::borrow_mut(&mut reg.subjects, &subject_addr);
                profile.challenges_failed = profile.challenges_failed + 1;
                // Reputation penalty
                if (profile.reputation_score >= 50) {
                    profile.reputation_score = profile.reputation_score - 50;
                } else {
                    profile.reputation_score = 0;
                };

                event::emit(ChallengeFailed {
                    challenge_id,
                    subject: subject_addr,
                    claimed_domain: domain,
                });
            };
        };
    }

    // ===================== Expire stale challenges =====================

    public entry fun expire_challenge(
        _anyone: &signer,
        registry_addr: address,
        challenge_id: u64,
    ) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global_mut<ChallengeRegistry>(registry_addr);
        assert!(table::contains(&reg.challenges, challenge_id), E_CHALLENGE_NOT_FOUND);

        let challenge = table::borrow_mut(&mut reg.challenges, challenge_id);
        let now = timestamp::now_seconds();
        assert!(now >= challenge.expires_at, E_CHALLENGE_NOT_OPEN);
        assert!(challenge.status < STATUS_PASSED, E_ALREADY_CLAIMED);

        challenge.status = STATUS_EXPIRED;
        reg.total_failed = reg.total_failed + 1;

        let subject_addr = challenge.subject;
        if (simple_map::contains_key(&reg.subjects, &subject_addr)) {
            let profile = simple_map::borrow_mut(&mut reg.subjects, &subject_addr);
            profile.challenges_failed = profile.challenges_failed + 1;
        };
    }

    // ===================== View functions =====================

    #[view]
    public fun get_challenge(
        registry_addr: address,
        challenge_id: u64,
    ): (address, address, String, String, u64, u8) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global<ChallengeRegistry>(registry_addr);
        assert!(table::contains(&reg.challenges, challenge_id), E_CHALLENGE_NOT_FOUND);

        let c = table::borrow(&reg.challenges, challenge_id);
        (c.challenger, c.subject, c.claimed_domain, c.repo, c.issue_number, c.status)
    }

    #[view]
    public fun get_subject_profile(
        registry_addr: address,
        subject: address,
    ): (u64, u64, u64, u64) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global<ChallengeRegistry>(registry_addr);
        assert!(simple_map::contains_key(&reg.subjects, &subject), E_SUBJECT_NOT_FOUND);

        let p = simple_map::borrow(&reg.subjects, &subject);
        (p.challenges_received, p.challenges_passed, p.challenges_failed, p.reputation_score)
    }

    #[view]
    public fun get_registry_stats(
        registry_addr: address,
    ): (u64, u64, u64) acquires ChallengeRegistry {
        assert!(exists<ChallengeRegistry>(registry_addr), E_REGISTRY_NOT_FOUND);
        let reg = borrow_global<ChallengeRegistry>(registry_addr);
        (reg.challenge_count, reg.total_passed, reg.total_failed)
    }

    // ===================== Helpers =====================

    fun find_reviewer_index(reviewers: &vector<address>, reviewer: address): u64 {
        let i = 0;
        let len = vector::length(reviewers);
        while (i < len) {
            if (*vector::borrow(reviewers, i) == reviewer) {
                return i
            };
            i = i + 1;
        };
        999 // sentinel: not found
    }

    fun vector_contains(v: &vector<String>, item: &String): bool {
        let i = 0;
        let len = vector::length(v);
        while (i < len) {
            if (vector::borrow(v, i) == item) {
                return true
            };
            i = i + 1;
        };
        false
    }
}
