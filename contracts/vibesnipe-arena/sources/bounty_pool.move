module vibesnipe_arena::bounty_pool {
    use std::signer;
    use aptos_framework::event;
    use aptos_framework::coin::{Self, Coin};
    use aptos_framework::aptos_coin::AptosCoin;
    use aptos_std::table::{Self, Table};

    friend vibesnipe_arena::vibesnipe_arena;

    // ===================== Error codes =====================

    const E_POOL_EXISTS: u64 = 100;
    const E_POOL_NOT_FOUND: u64 = 101;
    const E_ZERO_DEPOSIT: u64 = 102;
    const E_INSUFFICIENT_SHARES: u64 = 103;
    const E_INSUFFICIENT_POOL: u64 = 104;
    const E_NOT_DEPOSITOR: u64 = 105;
    const E_ZERO_WITHDRAW: u64 = 106;
    const E_NOT_ADMIN: u64 = 107;

    // Fee basis points: 50 = 0.5% protocol fee on payouts
    const PROTOCOL_FEE_BPS: u64 = 50;
    const BPS_DENOMINATOR: u64 = 10000;

    // ===================== Structs =====================

    struct LPPosition has store, copy, drop {
        shares: u64,
        deposited: u64,
        earned_fees: u64,
    }

    struct BountyPool has key {
        admin: address,
        vault: Coin<AptosCoin>,
        total_shares: u64,
        total_deposited: u64,
        total_paid_out: u64,
        total_fees_earned: u64,
        protocol_fees: u64,
        positions: Table<address, LPPosition>,
        depositor_count: u64,
    }

    // ===================== Events =====================

    #[event]
    struct Deposited has drop, store {
        depositor: address,
        amount: u64,
        shares_minted: u64,
        total_pool: u64,
    }

    #[event]
    struct Withdrawn has drop, store {
        depositor: address,
        amount: u64,
        shares_burned: u64,
        total_pool: u64,
    }

    #[event]
    struct BountyPaid has drop, store {
        recipient: address,
        exploit_id: u64,
        gross_amount: u64,
        fee: u64,
        net_amount: u64,
    }

    #[event]
    struct FeesDistributed has drop, store {
        total_fees: u64,
    }

    // ===================== Init =====================

    public entry fun initialize(admin: &signer) {
        let admin_addr = signer::address_of(admin);
        assert!(!exists<BountyPool>(admin_addr), E_POOL_EXISTS);

        move_to(admin, BountyPool {
            admin: admin_addr,
            vault: coin::zero<AptosCoin>(),
            total_shares: 0,
            total_deposited: 0,
            total_paid_out: 0,
            total_fees_earned: 0,
            protocol_fees: 0,
            positions: table::new(),
            depositor_count: 0,
        });
    }

    // ===================== LP deposit =====================

    public entry fun deposit(
        depositor: &signer,
        pool_addr: address,
        amount: u64,
    ) acquires BountyPool {
        assert!(amount > 0, E_ZERO_DEPOSIT);
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);

        let pool = borrow_global_mut<BountyPool>(pool_addr);
        let dep_addr = signer::address_of(depositor);

        // Mint shares: if pool is empty, 1:1. Otherwise proportional to vault.
        let vault_value = coin::value(&pool.vault);
        let shares_to_mint = if (pool.total_shares == 0 || vault_value == 0) {
            amount
        } else {
            (amount * pool.total_shares) / vault_value
        };

        // Transfer APT into vault
        let coins = coin::withdraw<AptosCoin>(depositor, amount);
        coin::merge(&mut pool.vault, coins);

        pool.total_shares = pool.total_shares + shares_to_mint;
        pool.total_deposited = pool.total_deposited + amount;

        // Update or create position
        if (table::contains(&pool.positions, dep_addr)) {
            let pos = table::borrow_mut(&mut pool.positions, dep_addr);
            pos.shares = pos.shares + shares_to_mint;
            pos.deposited = pos.deposited + amount;
        } else {
            table::add(&mut pool.positions, dep_addr, LPPosition {
                shares: shares_to_mint,
                deposited: amount,
                earned_fees: 0,
            });
            pool.depositor_count = pool.depositor_count + 1;
        };

        event::emit(Deposited {
            depositor: dep_addr,
            amount,
            shares_minted: shares_to_mint,
            total_pool: coin::value(&pool.vault),
        });
    }

    // ===================== LP withdraw =====================

    public entry fun withdraw(
        depositor: &signer,
        pool_addr: address,
        shares_to_burn: u64,
    ) acquires BountyPool {
        assert!(shares_to_burn > 0, E_ZERO_WITHDRAW);
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);

        let pool = borrow_global_mut<BountyPool>(pool_addr);
        let dep_addr = signer::address_of(depositor);
        assert!(table::contains(&pool.positions, dep_addr), E_NOT_DEPOSITOR);

        let pos = table::borrow_mut(&mut pool.positions, dep_addr);
        assert!(pos.shares >= shares_to_burn, E_INSUFFICIENT_SHARES);

        // Calculate proportional withdrawal amount
        let vault_value = coin::value(&pool.vault);
        let withdraw_amount = (shares_to_burn * vault_value) / pool.total_shares;
        assert!(withdraw_amount <= vault_value, E_INSUFFICIENT_POOL);

        // Burn shares
        pos.shares = pos.shares - shares_to_burn;
        pool.total_shares = pool.total_shares - shares_to_burn;

        // Transfer APT back to depositor
        let coins = coin::extract(&mut pool.vault, withdraw_amount);
        coin::deposit(dep_addr, coins);

        event::emit(Withdrawn {
            depositor: dep_addr,
            amount: withdraw_amount,
            shares_burned: shares_to_burn,
            total_pool: coin::value(&pool.vault),
        });
    }

    // ===================== Bounty payout (called by arena) =====================

    public(friend) fun pay_bounty(
        pool_addr: address,
        recipient: address,
        exploit_id: u64,
        gross_amount: u64,
    ): u64 acquires BountyPool {
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);

        let pool = borrow_global_mut<BountyPool>(pool_addr);
        let vault_value = coin::value(&pool.vault);
        assert!(gross_amount <= vault_value, E_INSUFFICIENT_POOL);

        // Calculate protocol fee
        let fee = (gross_amount * PROTOCOL_FEE_BPS) / BPS_DENOMINATOR;
        let net_amount = gross_amount - fee;

        // Pay recipient
        let payout_coins = coin::extract(&mut pool.vault, net_amount);
        coin::deposit(recipient, payout_coins);

        // Fee stays in pool (accrues to LPs)
        pool.total_paid_out = pool.total_paid_out + net_amount;
        pool.total_fees_earned = pool.total_fees_earned + fee;
        pool.protocol_fees = pool.protocol_fees + fee;

        event::emit(BountyPaid {
            recipient,
            exploit_id,
            gross_amount,
            fee,
            net_amount,
        });

        net_amount
    }

    // ===================== View functions =====================

    #[view]
    public fun get_pool_stats(pool_addr: address): (u64, u64, u64, u64, u64, u64) acquires BountyPool {
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);
        let pool = borrow_global<BountyPool>(pool_addr);

        (
            coin::value(&pool.vault),
            pool.total_shares,
            pool.total_deposited,
            pool.total_paid_out,
            pool.total_fees_earned,
            pool.depositor_count,
        )
    }

    #[view]
    public fun get_position(pool_addr: address, depositor: address): (u64, u64, u64) acquires BountyPool {
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);
        let pool = borrow_global<BountyPool>(pool_addr);
        assert!(table::contains(&pool.positions, depositor), E_NOT_DEPOSITOR);

        let pos = table::borrow(&pool.positions, depositor);
        (pos.shares, pos.deposited, pos.earned_fees)
    }

    #[view]
    public fun share_value(pool_addr: address, shares: u64): u64 acquires BountyPool {
        assert!(exists<BountyPool>(pool_addr), E_POOL_NOT_FOUND);
        let pool = borrow_global<BountyPool>(pool_addr);

        if (pool.total_shares == 0) {
            0
        } else {
            (shares * coin::value(&pool.vault)) / pool.total_shares
        }
    }

    #[view]
    public fun pool_exists(pool_addr: address): bool {
        exists<BountyPool>(pool_addr)
    }
}
