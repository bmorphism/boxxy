#[test_only]
module vibesnipe_arena::credential_challenge_tests {
    use std::string;
    use aptos_framework::account;
    use aptos_framework::timestamp;
    use vibesnipe_arena::credential_challenge;

    fun setup(aptos_framework: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
        timestamp::fast_forward_seconds(1000);
        account::create_account_for_test(@0xCAFE);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE)]
    fun test_initialize(aptos_framework: &signer, admin: &signer) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);

        let (count, passed, failed) = credential_challenge::get_registry_stats(@0xCAFE);
        assert!(count == 0, 0);
        assert!(passed == 0, 1);
        assert!(failed == 0, 2);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE)]
    #[expected_failure(abort_code = 200)] // E_REGISTRY_EXISTS
    fun test_double_init(aptos_framework: &signer, admin: &signer) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        credential_challenge::initialize(admin);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, sniper = @0xA, expert = @0xB)]
    fun test_issue_challenge(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        credential_challenge::issue_challenge(
            sniper,
            @0xCAFE,
            @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1234,
            string::utf8(b"Fix pipeline caching for KFP v2"),
        );

        let (challenger, subject, domain, repo, issue, status) =
            credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(challenger == @0xA, 10);
        assert!(subject == @0xB, 11);
        assert!(domain == string::utf8(b"kubeflow"), 12);
        assert!(repo == string::utf8(b"kubeflow/kubeflow"), 13);
        assert!(issue == 1234, 14);
        assert!(status == 0, 15); // STATUS_OPEN

        let (received, passed, failed, rep) =
            credential_challenge::get_subject_profile(@0xCAFE, @0xB);
        assert!(received == 1, 16);
        assert!(passed == 0, 17);
        assert!(failed == 0, 18);
        assert!(rep == 0, 19);
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, sniper = @0xA, expert = @0xB)]
    fun test_accept_challenge(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1234,
            string::utf8(b"Fix pipeline caching"),
        );

        credential_challenge::accept_challenge(expert, @0xCAFE, 0);

        let (_c, _s, _d, _r, _i, status) = credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(status == 1, 20); // STATUS_ACCEPTED
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, sniper = @0xA, expert = @0xB)]
    fun test_submit_pr(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1234,
            string::utf8(b"Fix pipeline caching"),
        );
        credential_challenge::accept_challenge(expert, @0xCAFE, 0);
        credential_challenge::submit_pr(expert, @0xCAFE, 0, 5678, x"abc123def456");

        let (_c, _s, _d, _r, _i, status) = credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(status == 2, 30); // STATUS_SUBMITTED
    }

    #[test(
        aptos_framework = @0x1, admin = @0xCAFE,
        sniper = @0xA, expert = @0xB,
        r1 = @0x10, r2 = @0x11, r3 = @0x12
    )]
    fun test_full_vibesniping_pass(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
        r1: &signer,
        r2: &signer,
        r3: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0x10);
        account::create_account_for_test(@0x11);
        account::create_account_for_test(@0x12);

        // Register 3 reviewers (triadic)
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x10);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x11);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x12);

        // Sniper vibesnipts the "kubeflow expert"
        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1234,
            string::utf8(b"Fix pipeline caching for KFP v2"),
        );

        // Expert accepts and submits PR
        credential_challenge::accept_challenge(expert, @0xCAFE, 0);
        credential_challenge::submit_pr(expert, @0xCAFE, 0, 5678, x"abc123");

        // Triadic review: all 3 approve
        credential_challenge::review_challenge(r1, @0xCAFE, 0, 1); // approve
        credential_challenge::review_challenge(r2, @0xCAFE, 0, 1); // approve
        credential_challenge::review_challenge(r3, @0xCAFE, 0, 1); // approve

        // Challenge passed
        let (_c, _s, _d, _r, _i, status) = credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(status == 4, 40); // STATUS_PASSED

        // Expert earned reputation
        let (received, passed, failed, rep) =
            credential_challenge::get_subject_profile(@0xCAFE, @0xB);
        assert!(received == 1, 41);
        assert!(passed == 1, 42);
        assert!(failed == 0, 43);
        assert!(rep == 100, 44);

        // Registry stats
        let (count, total_passed, total_failed) =
            credential_challenge::get_registry_stats(@0xCAFE);
        assert!(count == 1, 45);
        assert!(total_passed == 1, 46);
        assert!(total_failed == 0, 47);
    }

    #[test(
        aptos_framework = @0x1, admin = @0xCAFE,
        sniper = @0xA, expert = @0xB,
        r1 = @0x10, r2 = @0x11, r3 = @0x12
    )]
    fun test_vibesniping_fail_majority_reject(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
        r1: &signer,
        r2: &signer,
        r3: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0x10);
        account::create_account_for_test(@0x11);
        account::create_account_for_test(@0x12);

        credential_challenge::add_reviewer(admin, @0xCAFE, @0x10);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x11);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x12);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            999,
            string::utf8(b"Implement distributed training"),
        );

        credential_challenge::accept_challenge(expert, @0xCAFE, 0);
        credential_challenge::submit_pr(expert, @0xCAFE, 0, 1111, x"deadbeef");

        // 2 reject, 1 approve -> FAILED
        credential_challenge::review_challenge(r1, @0xCAFE, 0, 1); // approve
        credential_challenge::review_challenge(r2, @0xCAFE, 0, 2); // reject
        credential_challenge::review_challenge(r3, @0xCAFE, 0, 2); // reject

        let (_c, _s, _d, _r, _i, status) = credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(status == 5, 50); // STATUS_FAILED

        let (_received, _passed, failed, rep) =
            credential_challenge::get_subject_profile(@0xCAFE, @0xB);
        assert!(failed == 1, 51);
        assert!(rep == 0, 52); // started at 0, penalty clamped to 0
    }

    #[test(
        aptos_framework = @0x1, admin = @0xCAFE,
        sniper = @0xA, expert = @0xB,
        r1 = @0x10, r2 = @0x11, r3 = @0x12
    )]
    #[expected_failure(abort_code = 209)] // E_ALREADY_REVIEWED
    fun test_double_review(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        expert: &signer,
        r1: &signer,
        r2: &signer,
        r3: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0x10);
        account::create_account_for_test(@0x11);
        account::create_account_for_test(@0x12);

        credential_challenge::add_reviewer(admin, @0xCAFE, @0x10);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x11);
        credential_challenge::add_reviewer(admin, @0xCAFE, @0x12);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            42,
            string::utf8(b"Some issue"),
        );

        credential_challenge::accept_challenge(expert, @0xCAFE, 0);
        credential_challenge::submit_pr(expert, @0xCAFE, 0, 100, x"aa");

        credential_challenge::review_challenge(r1, @0xCAFE, 0, 1);
        credential_challenge::review_challenge(r1, @0xCAFE, 0, 1); // double review
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, sniper = @0xA, wrong = @0xC)]
    #[expected_failure(abort_code = 206)] // E_NOT_SUBJECT
    fun test_wrong_person_accepts(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        wrong: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0xC);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1, string::utf8(b"x"),
        );

        credential_challenge::accept_challenge(wrong, @0xCAFE, 0); // not @0xB
    }

    #[test(aptos_framework = @0x1, admin = @0xCAFE, sniper = @0xA, anyone = @0xC)]
    fun test_expire_challenge(
        aptos_framework: &signer,
        admin: &signer,
        sniper: &signer,
        anyone: &signer,
    ) {
        setup(aptos_framework);
        credential_challenge::initialize(admin);
        account::create_account_for_test(@0xA);
        account::create_account_for_test(@0xB);
        account::create_account_for_test(@0xC);

        credential_challenge::issue_challenge(
            sniper, @0xCAFE, @0xB,
            string::utf8(b"kubeflow"),
            string::utf8(b"kubeflow/kubeflow"),
            1, string::utf8(b"x"),
        );

        // Fast forward past TTL (7 days + buffer)
        timestamp::fast_forward_seconds(700000);

        credential_challenge::expire_challenge(anyone, @0xCAFE, 0);

        let (_c, _s, _d, _r, _i, status) = credential_challenge::get_challenge(@0xCAFE, 0);
        assert!(status == 6, 60); // STATUS_EXPIRED
    }
}
