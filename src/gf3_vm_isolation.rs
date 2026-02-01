//! GF(3) VM Isolation for Boxxy macOS Virtualization
//!
//! Maps isolation levels to GF(3) trits for balanced capability flow:
//! - MINUS (-1): Sandbox - least privileged, maximum restriction
//! - ERGODIC (0): Container - balanced isolation, coordinated access
//! - PLUS (+1): VM - full isolation, maximum privilege within boundary

use std::collections::HashMap;
use std::fmt;

/// GF(3) trit values for isolation levels
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(i8)]
pub enum Trit {
    Minus = -1,
    Ergodic = 0,
    Plus = 1,
}

impl Trit {
    pub fn from_i8(v: i8) -> Self {
        match v.rem_euclid(3) {
            0 => Trit::Ergodic,
            1 => Trit::Plus,
            2 => Trit::Minus,
            _ => unreachable!(),
        }
    }

    pub fn add(self, other: Trit) -> Trit {
        Trit::from_i8(self as i8 + other as i8)
    }

    pub fn neg(self) -> Trit {
        Trit::from_i8(-(self as i8))
    }
}

impl fmt::Display for Trit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Trit::Minus => write!(f, "MINUS(-1)"),
            Trit::Ergodic => write!(f, "ERGODIC(0)"),
            Trit::Plus => write!(f, "PLUS(+1)"),
        }
    }
}

/// Isolation levels mapped to GF(3)
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum IsolationLevel {
    /// Sandbox: MINUS (-1) - App Sandbox, minimal capabilities
    Sandbox,
    /// Container: ERGODIC (0) - Container-level isolation, balanced
    Container,
    /// VM: PLUS (+1) - Full VM isolation via Virtualization.framework
    VM,
}

impl IsolationLevel {
    pub fn trit(&self) -> Trit {
        match self {
            IsolationLevel::Sandbox => Trit::Minus,
            IsolationLevel::Container => Trit::Ergodic,
            IsolationLevel::VM => Trit::Plus,
        }
    }

    pub fn from_trit(t: Trit) -> Self {
        match t {
            Trit::Minus => IsolationLevel::Sandbox,
            Trit::Ergodic => IsolationLevel::Container,
            Trit::Plus => IsolationLevel::VM,
        }
    }

    pub fn privilege_level(&self) -> u8 {
        match self {
            IsolationLevel::Sandbox => 1,
            IsolationLevel::Container => 2,
            IsolationLevel::VM => 3,
        }
    }
}

impl fmt::Display for IsolationLevel {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            IsolationLevel::Sandbox => write!(f, "Sandbox[MINUS]"),
            IsolationLevel::Container => write!(f, "Container[ERGODIC]"),
            IsolationLevel::VM => write!(f, "VM[PLUS]"),
        }
    }
}

/// Isolation policy with GF(3) trit assignment
#[derive(Debug, Clone)]
pub struct IsolationPolicy {
    pub name: String,
    pub level: IsolationLevel,
    pub trit: Trit,
    pub allowed_capabilities: Vec<String>,
    pub denied_capabilities: Vec<String>,
}

impl IsolationPolicy {
    pub fn new(name: impl Into<String>, level: IsolationLevel) -> Self {
        let trit = level.trit();
        let (allowed, denied) = Self::default_capabilities(level);
        Self {
            name: name.into(),
            level,
            trit,
            allowed_capabilities: allowed,
            denied_capabilities: denied,
        }
    }

    fn default_capabilities(level: IsolationLevel) -> (Vec<String>, Vec<String>) {
        match level {
            IsolationLevel::Sandbox => (
                vec!["read-user-data".into(), "network-client".into()],
                vec![
                    "hardware-access".into(),
                    "kernel-extension".into(),
                    "system-modification".into(),
                ],
            ),
            IsolationLevel::Container => (
                vec![
                    "read-user-data".into(),
                    "write-user-data".into(),
                    "network-client".into(),
                    "network-server".into(),
                ],
                vec!["kernel-extension".into(), "system-modification".into()],
            ),
            IsolationLevel::VM => (
                vec![
                    "virtualization".into(),
                    "hardware-emulation".into(),
                    "network-bridge".into(),
                    "disk-image".into(),
                ],
                vec![],
            ),
        }
    }

    pub fn can_capability(&self, cap: &str) -> bool {
        self.allowed_capabilities.iter().any(|c| c == cap)
            && !self.denied_capabilities.iter().any(|c| c == cap)
    }
}

/// Capability crossing record for boundary verification
#[derive(Debug, Clone)]
pub struct CapabilityCrossing {
    pub capability: String,
    pub from_level: IsolationLevel,
    pub to_level: IsolationLevel,
    pub direction_trit: Trit,
}

impl CapabilityCrossing {
    pub fn new(capability: impl Into<String>, from: IsolationLevel, to: IsolationLevel) -> Self {
        let direction_trit = to.trit().add(from.trit().neg());
        Self {
            capability: capability.into(),
            from_level: from,
            to_level: to,
            direction_trit,
        }
    }
}

/// Security boundary enforcing GF(3) balanced capability flow
#[derive(Debug)]
pub struct SecurityBoundary {
    policies: HashMap<String, IsolationPolicy>,
    crossings: Vec<CapabilityCrossing>,
    balance_sum: i8,
}

impl SecurityBoundary {
    pub fn new() -> Self {
        Self {
            policies: HashMap::new(),
            crossings: Vec::new(),
            balance_sum: 0,
        }
    }

    pub fn register_policy(&mut self, policy: IsolationPolicy) {
        self.policies.insert(policy.name.clone(), policy);
    }

    /// Record a capability crossing between isolation levels
    pub fn cross_boundary(
        &mut self,
        capability: impl Into<String>,
        from: IsolationLevel,
        to: IsolationLevel,
    ) -> Result<(), SecurityError> {
        let crossing = CapabilityCrossing::new(capability, from, to);

        if let Some(from_policy) = self.find_policy_for_level(from) {
            if !from_policy.can_capability(&crossing.capability) {
                return Err(SecurityError::CapabilityDenied {
                    capability: crossing.capability,
                    level: from,
                });
            }
        }

        self.balance_sum += crossing.direction_trit as i8;
        self.crossings.push(crossing);
        Ok(())
    }

    fn find_policy_for_level(&self, level: IsolationLevel) -> Option<&IsolationPolicy> {
        self.policies.values().find(|p| p.level == level)
    }

    /// Verify that capability flow is GF(3) balanced (sum ≡ 0 mod 3)
    pub fn verify_capability_balance(&self) -> BalanceResult {
        let sum_mod3 = self.balance_sum.rem_euclid(3);
        let balanced = sum_mod3 == 0;

        BalanceResult {
            balanced,
            sum: self.balance_sum,
            sum_mod3,
            crossings_count: self.crossings.len(),
            correction_needed: if balanced {
                None
            } else {
                Some(Trit::from_i8(-self.balance_sum))
            },
        }
    }

    /// Get all crossings for audit
    pub fn audit_crossings(&self) -> &[CapabilityCrossing] {
        &self.crossings
    }

    /// Apply a corrective crossing to restore balance
    pub fn apply_correction(&mut self, correction: Trit) {
        self.balance_sum += correction as i8;
    }
}

impl Default for SecurityBoundary {
    fn default() -> Self {
        Self::new()
    }
}

/// Result of balance verification
#[derive(Debug, Clone)]
pub struct BalanceResult {
    pub balanced: bool,
    pub sum: i8,
    pub sum_mod3: i8,
    pub crossings_count: usize,
    pub correction_needed: Option<Trit>,
}

impl fmt::Display for BalanceResult {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.balanced {
            write!(
                f,
                "✓ BALANCED: {} crossings, sum={} ≡ 0 (mod 3)",
                self.crossings_count, self.sum
            )
        } else {
            write!(
                f,
                "✗ UNBALANCED: {} crossings, sum={} ≡ {} (mod 3), needs {:?}",
                self.crossings_count,
                self.sum,
                self.sum_mod3,
                self.correction_needed
            )
        }
    }
}

/// Security errors
#[derive(Debug, Clone)]
pub enum SecurityError {
    CapabilityDenied {
        capability: String,
        level: IsolationLevel,
    },
    UnbalancedFlow {
        sum: i8,
    },
    PolicyNotFound {
        name: String,
    },
}

impl fmt::Display for SecurityError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SecurityError::CapabilityDenied { capability, level } => {
                write!(f, "Capability '{}' denied at {}", capability, level)
            }
            SecurityError::UnbalancedFlow { sum } => {
                write!(f, "Unbalanced capability flow: sum={} ≢ 0 (mod 3)", sum)
            }
            SecurityError::PolicyNotFound { name } => {
                write!(f, "Policy not found: {}", name)
            }
        }
    }
}

impl std::error::Error for SecurityError {}

/// Convenience builder for VM isolation configurations
pub struct IsolationBuilder {
    boundary: SecurityBoundary,
}

impl IsolationBuilder {
    pub fn new() -> Self {
        Self {
            boundary: SecurityBoundary::new(),
        }
    }

    pub fn with_sandbox(mut self, name: impl Into<String>) -> Self {
        self.boundary
            .register_policy(IsolationPolicy::new(name, IsolationLevel::Sandbox));
        self
    }

    pub fn with_container(mut self, name: impl Into<String>) -> Self {
        self.boundary
            .register_policy(IsolationPolicy::new(name, IsolationLevel::Container));
        self
    }

    pub fn with_vm(mut self, name: impl Into<String>) -> Self {
        self.boundary
            .register_policy(IsolationPolicy::new(name, IsolationLevel::VM));
        self
    }

    pub fn build(self) -> SecurityBoundary {
        self.boundary
    }
}

impl Default for IsolationBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_trit_arithmetic() {
        assert_eq!(Trit::Plus.add(Trit::Minus), Trit::Ergodic);
        assert_eq!(Trit::Plus.add(Trit::Plus), Trit::Minus);
        assert_eq!(Trit::Minus.neg(), Trit::Plus);
    }

    #[test]
    fn test_isolation_level_mapping() {
        assert_eq!(IsolationLevel::Sandbox.trit(), Trit::Minus);
        assert_eq!(IsolationLevel::Container.trit(), Trit::Ergodic);
        assert_eq!(IsolationLevel::VM.trit(), Trit::Plus);
    }

    #[test]
    fn test_balanced_crossing() {
        let mut boundary = IsolationBuilder::new()
            .with_sandbox("app-sandbox")
            .with_vm("honeypot-vm")
            .build();

        boundary
            .cross_boundary("virtualization", IsolationLevel::Sandbox, IsolationLevel::VM)
            .ok();
        boundary
            .cross_boundary("read-user-data", IsolationLevel::VM, IsolationLevel::Sandbox)
            .ok();

        let result = boundary.verify_capability_balance();
        assert!(result.balanced, "Opposite crossings should balance");
    }

    #[test]
    fn test_unbalanced_crossing() {
        let mut boundary = SecurityBoundary::new();
        boundary
            .cross_boundary("cap1", IsolationLevel::Sandbox, IsolationLevel::VM)
            .ok();

        let result = boundary.verify_capability_balance();
        assert!(!result.balanced);
        assert!(result.correction_needed.is_some());
    }

    #[test]
    fn test_triadic_balance() {
        let mut boundary = SecurityBoundary::new();

        boundary
            .cross_boundary("a", IsolationLevel::Sandbox, IsolationLevel::Container)
            .ok();
        boundary
            .cross_boundary("b", IsolationLevel::Container, IsolationLevel::VM)
            .ok();
        boundary
            .cross_boundary("c", IsolationLevel::VM, IsolationLevel::Sandbox)
            .ok();

        let result = boundary.verify_capability_balance();
        assert!(result.balanced, "Full cycle should balance: S→C→V→S");
    }
}
