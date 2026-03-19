#[test_only]
module vibesnipe_arena::vibesnipe_arena_tests {
    use std::string;
    use aptos_framework::account;
    use aptos_framework::timestamp;
    use aptos_framework::coin;
    use aptos_framework::aptos_coin::{Self, AptosCoin};
    use vibesnipe_arena::vibesnipe_arena;
    use vibesnipe_arena::bounty_pool;

    fun setup_test(aptos_framework: &signer, _admin: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
        timestamp::fast_forward_seconds(1000);

        account::create_account_for_test(@0xCAFE);
    }

    fun setup_test_with_coins(aptos_framework: &signer, _admin: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
        timestamp::fast_forward_seconds(1000);

        account::create_account_for_test(@0xCAFE);
        let (burn_cap, mint_cap) = aptos_coin::initialize_for_test(aptos_framework);
        coin::destroy_burn_cap(burn_cap);
        coin::destroy_mint_cap(mint_cap);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE)]
    fun test_initialize(aptos_framework: &signer, admin: &signer) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE)]
    #[expected_failure(abort_code = 2)] // E_MARKETPLACE_EXISTS
    fun test_double_initialize(aptos_framework: &signer, admin: &signer) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC)]
    fun test_register_runtimes_gf3_balanced(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);

        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0xC);

        // MINUS(-1), ERGODIC(0), PLUS(+1) -> balanced
        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"validator-v1"),
            string::utf8(b"1.0.0"),
            0, // TRIT_MINUS
        );
        vibesnipe_arena::register_runtime(
            op2, @0xCAFE,
            string::utf8(b"coordinator-v2"),
            string::utf8(b"2.1.0"),
            1, // TRIT_ERGODIC
        );
        vibesnipe_arena::register_runtime(
            op3, @0xCAFE,
            string::utf8(b"generator-v3"),
            string::utf8(b"3.2.0"),
            2, // TRIT_PLUS
        );

        assert!(vibesnipe_arena::verify_gf3_balance(@0xCAFE), 100);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA)]
    #[expected_failure(abort_code = 6)] // E_INVALID_TRIT
    fun test_invalid_trit(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        account::create_account_for_test(@0xA);

        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"bad-runtime"),
            string::utf8(b"1.0.0"),
            5, // invalid trit
        );
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC)]
    fun test_start_round(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        register_balanced_triad(op1, op2, op3);

        vibesnipe_arena::start_round(admin, @0xCAFE);

        let (_runtimes, _exploits, round, balanced, active) =
            vibesnipe_arena::get_arena_stats(@0xCAFE);
        assert!(round == 1, 200);
        assert!(balanced, 201);
        assert!(active, 202);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC, attacker = @0xD)]
    fun test_submit_and_verify_exploit(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
        attacker: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        register_balanced_triad(op1, op2, op3);
        account::create_account_for_test(@0xD);

        vibesnipe_arena::start_round(admin, @0xCAFE);

        // Submit exploit
        let proof = x"deadbeefcafe1234";
        vibesnipe_arena::submit_exploit(
            attacker, @0xCAFE,
            string::utf8(b"validator-v1"),
            0, // CLASS_TIMING_ATTACK
            8, // severity
            proof,
        );

        // Check exploit exists but not yet verified
        let (_target, _class, severity, _submitter, verified) =
            vibesnipe_arena::get_exploit(@0xCAFE, 0);
        assert!(severity == 8, 300);
        assert!(!verified, 301);

        // Triadic consensus: all three runtimes vote
        vibesnipe_arena::vote_exploit(op1, @0xCAFE, 0, string::utf8(b"validator-v1"), true);
        vibesnipe_arena::vote_exploit(op2, @0xCAFE, 0, string::utf8(b"coordinator-v2"), true);
        vibesnipe_arena::vote_exploit(op3, @0xCAFE, 0, string::utf8(b"generator-v3"), true);

        // Now verified
        let (_target2, _class2, _severity2, _submitter2, verified) =
            vibesnipe_arena::get_exploit(@0xCAFE, 0);
        assert!(verified, 302);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC, attacker = @0xD)]
    fun test_exploit_rejected_on_nay(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
        attacker: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        register_balanced_triad(op1, op2, op3);
        account::create_account_for_test(@0xD);

        vibesnipe_arena::start_round(admin, @0xCAFE);

        vibesnipe_arena::submit_exploit(
            attacker, @0xCAFE,
            string::utf8(b"coordinator-v2"),
            5, // CLASS_CONSENSUS_BREAK
            9,
            x"aabbccdd",
        );

        // Validator says yes, coordinator says no, generator says yes
        vibesnipe_arena::vote_exploit(op1, @0xCAFE, 0, string::utf8(b"validator-v1"), true);
        vibesnipe_arena::vote_exploit(op2, @0xCAFE, 0, string::utf8(b"coordinator-v2"), false);
        vibesnipe_arena::vote_exploit(op3, @0xCAFE, 0, string::utf8(b"generator-v3"), true);

        // Should NOT be verified (coordinator vetoed)
        let (_target, _class, _severity, _submitter, verified) =
            vibesnipe_arena::get_exploit(@0xCAFE, 0);
        assert!(!verified, 400);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC, attacker = @0xD)]
    #[expected_failure(abort_code = 9)] // E_ALREADY_VOTED
    fun test_double_vote(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
        attacker: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        register_balanced_triad(op1, op2, op3);
        account::create_account_for_test(@0xD);

        vibesnipe_arena::start_round(admin, @0xCAFE);

        vibesnipe_arena::submit_exploit(
            attacker, @0xCAFE,
            string::utf8(b"generator-v3"),
            1, // CLASS_MEMORY_SIDECHANNEL
            7,
            x"1122",
        );

        vibesnipe_arena::vote_exploit(op1, @0xCAFE, 0, string::utf8(b"validator-v1"), true);
        vibesnipe_arena::vote_exploit(op1, @0xCAFE, 0, string::utf8(b"validator-v1"), true); // double vote
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB, op3 = @0xC)]
    fun test_arena_stats(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
        op3: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        register_balanced_triad(op1, op2, op3);

        let (runtimes, exploits, round, balanced, active) =
            vibesnipe_arena::get_arena_stats(@0xCAFE);
        assert!(runtimes == 3, 500);
        assert!(exploits == 0, 501);
        assert!(round == 0, 502);
        assert!(balanced, 503);
        assert!(!active, 504);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB)]
    fun test_gf3_imbalanced(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        // Only MINUS and PLUS, no ERGODIC -> might be balanced (1-1=0 mod 3)
        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"rt-minus"),
            string::utf8(b"1.0"),
            0,
        );
        vibesnipe_arena::register_runtime(
            op2, @0xCAFE,
            string::utf8(b"rt-plus"),
            string::utf8(b"1.0"),
            2,
        );

        // 1 MINUS, 1 PLUS -> plus_count(1) - minus_count(1) = 0 mod 3 -> balanced
        assert!(vibesnipe_arena::verify_gf3_balance(@0xCAFE), 600);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB)]
    fun test_gf3_truly_imbalanced(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        // Two PLUS runtimes, no MINUS -> plus(2) - minus(0) = 2, not ≡ 0 mod 3
        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"rt-plus-1"),
            string::utf8(b"1.0"),
            2,
        );
        vibesnipe_arena::register_runtime(
            op2, @0xCAFE,
            string::utf8(b"rt-plus-2"),
            string::utf8(b"1.0"),
            2,
        );

        assert!(!vibesnipe_arena::verify_gf3_balance(@0xCAFE), 700);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, op1 = @0xA, op2 = @0xB)]
    #[expected_failure(abort_code = 7)] // E_GF3_IMBALANCED
    fun test_start_round_imbalanced_fails(
        aptos_framework: &signer,
        admin: &signer,
        op1: &signer,
        op2: &signer,
    ) {
        setup_test(aptos_framework, admin);
        vibesnipe_arena::initialize(admin, 1_000_000, @0xCAFE);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"rt-plus-1"),
            string::utf8(b"1.0"),
            2,
        );
        vibesnipe_arena::register_runtime(
            op2, @0xCAFE,
            string::utf8(b"rt-plus-2"),
            string::utf8(b"1.0"),
            2,
        );

        vibesnipe_arena::start_round(admin, @0xCAFE); // fails: GF(3) imbalanced
    }

    // ===== Bounty Pool Tests =====

    #[test(aptos_framework = @0x1, admin = @0xCAFE)]
    fun test_bounty_pool_initialize(aptos_framework: &signer, admin: &signer) {
        setup_test(aptos_framework, admin);
        bounty_pool::initialize(admin);
        assert!(bounty_pool::pool_exists(@0xCAFE), 800);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, lp1 = @0xE)]
    fun test_bounty_pool_deposit(aptos_framework: &signer, admin: &signer, lp1: &signer) {
        setup_test_with_coins(aptos_framework, admin);
        bounty_pool::initialize(admin);

        account::create_account_for_test(@0xE);
        coin::register<AptosCoin>(lp1);
        aptos_coin::mint(aptos_framework, @0xE, 100_000_000); // 1 APT

        bounty_pool::deposit(lp1, @0xCAFE, 50_000_000); // 0.5 APT

        let (vault, shares, deposited, _paid, _fees, depositors) =
            bounty_pool::get_pool_stats(@0xCAFE);
        assert!(vault == 50_000_000, 900);
        assert!(shares == 50_000_000, 901);
        assert!(deposited == 50_000_000, 902);
        assert!(depositors == 1, 903);

        let (lp_shares, lp_deposited, _lp_fees) = bounty_pool::get_position(@0xCAFE, @0xE);
        assert!(lp_shares == 50_000_000, 904);
        assert!(lp_deposited == 50_000_000, 905);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, lp1 = @0xE)]
    fun test_bounty_pool_withdraw(aptos_framework: &signer, admin: &signer, lp1: &signer) {
        setup_test_with_coins(aptos_framework, admin);
        bounty_pool::initialize(admin);

        account::create_account_for_test(@0xE);
        coin::register<AptosCoin>(lp1);
        aptos_coin::mint(aptos_framework, @0xE, 100_000_000);

        bounty_pool::deposit(lp1, @0xCAFE, 100_000_000);
        bounty_pool::withdraw(lp1, @0xCAFE, 50_000_000); // withdraw half

        let (vault, shares, _deposited, _paid, _fees, _depositors) =
            bounty_pool::get_pool_stats(@0xCAFE);
        assert!(vault == 50_000_000, 1000);
        assert!(shares == 50_000_000, 1001);

        assert!(coin::balance<AptosCoin>(@0xE) == 50_000_000, 1002);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, lp1 = @0xE)]
    fun test_share_value(aptos_framework: &signer, admin: &signer, lp1: &signer) {
        setup_test_with_coins(aptos_framework, admin);
        bounty_pool::initialize(admin);

        account::create_account_for_test(@0xE);
        coin::register<AptosCoin>(lp1);
        aptos_coin::mint(aptos_framework, @0xE, 100_000_000);

        bounty_pool::deposit(lp1, @0xCAFE, 100_000_000);

        let val = bounty_pool::share_value(@0xCAFE, 100_000_000);
        assert!(val == 100_000_000, 1100);

        let half_val = bounty_pool::share_value(@0xCAFE, 50_000_000);
        assert!(half_val == 50_000_000, 1101);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, lp1 = @0xE, lp2 = @0xF)]
    fun test_multi_lp_deposit(
        aptos_framework: &signer,
        admin: &signer,
        lp1: &signer,
        lp2: &signer,
    ) {
        setup_test_with_coins(aptos_framework, admin);
        bounty_pool::initialize(admin);

        account::create_account_for_test(@0xE);
        account::create_account_for_test(@0xF);
        coin::register<AptosCoin>(lp1);
        coin::register<AptosCoin>(lp2);
        aptos_coin::mint(aptos_framework, @0xE, 100_000_000);
        aptos_coin::mint(aptos_framework, @0xF, 200_000_000);

        bounty_pool::deposit(lp1, @0xCAFE, 100_000_000);
        bounty_pool::deposit(lp2, @0xCAFE, 200_000_000);

        let (vault, _shares, _deposited, _paid, _fees, depositors) =
            bounty_pool::get_pool_stats(@0xCAFE);
        assert!(vault == 300_000_000, 1200);
        assert!(depositors == 2, 1201);
    }

    // ===== Helpers =====

    fun register_balanced_triad(op1: &signer, op2: &signer, op3: &signer) {
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0xC);

        vibesnipe_arena::register_runtime(
            op1, @0xCAFE,
            string::utf8(b"validator-v1"),
            string::utf8(b"1.0.0"),
            0,
        );
        vibesnipe_arena::register_runtime(
            op2, @0xCAFE,
            string::utf8(b"coordinator-v2"),
            string::utf8(b"2.1.0"),
            1,
        );
        vibesnipe_arena::register_runtime(
            op3, @0xCAFE,
            string::utf8(b"generator-v3"),
            string::utf8(b"3.2.0"),
            2,
        );
    }
}
