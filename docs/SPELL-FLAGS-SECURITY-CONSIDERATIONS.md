# Spell.dbc Boolean Flags as Server Capability Witnessing Surface

## The RCE Vector: Warden Protocol

The 3.3.5a WoW client contains an architectural Remote Code Execution vector
in the Warden anti-cheat protocol. The server sends `SMSG_WARDEN_DATA` packets
containing **native x86 modules** that the client loads and executes. The
protocol flow:

```
Server                              Client (3.3.5a binary)
  |                                    |
  |--- SMSG_WARDEN_DATA (module) ----->|
  |    [encrypted x86 native code]     |  client LOADS and EXECUTES
  |                                    |  the payload in its own process
  |<--- CMSG_WARDEN_DATA (result) -----|
  |    [scan results: memory,          |
  |     page checksums, driver list]   |
```

**This is server-to-client RCE by design.** The client trusts whatever the
server sends as a Warden module. There is no signature verification against
a known Blizzard key. Any server operator -- or anyone who compromises the
server -- can push arbitrary native code to every connected client.

## July 2024: Turtle WoW Incident

In July 2024, Turtle WoW (a 3.3.5a-based private server with ~8000 concurrent
players) was compromised. The attack chain:

1. Attacker gained access to the server infrastructure
2. Server-side access => ability to craft `SMSG_WARDEN_DATA` payloads
3. Every connected client would execute whatever payload the server pushed

The community response revealed the actual security posture:

- A client-side patch was published that NOPs the Warden module loader
- **Warmane** (largest 3.3.5a private server, ~12000 concurrent) told players:
  > "Do not apply that fix or else you will not be able to play on Warmane
  > realms. The client will crash."
  >
  > "No further topics about this subject will be allowed in the forums."
  >
  > -- Warmane moderator Palutena, July 31 2024

This is the security equivalent of telling users not to patch a known RCE
because it would break the server's ability to run code on their machines.

## What The Spell Flags Witness

The 644 boolean flags in `spell_boolean_flags.json` are the **observable
surface** through which a client can determine what kind of server it is
connected to. Each flag is a bit of information the server has committed to
about its own behavior.

### Flags That Directly Touch the RCE Surface

| Flag | Security Relevance |
|------|-------------------|
| `SPELL_ATTR0_SERVER_ONLY` (0x00001000) | Spells that execute only server-side. The client never sees the effect logic. A compromised server can make ANY spell server-only and inject behavior. |
| `SPELL_ATTR0_NO_IMMUNITIES` (0x20000000) | Pierces invulnerability. When a server marks a spell with this flag, it declares the spell bypasses ALL client-side protections. Analogous to Warden bypassing OS security boundaries. |
| `SPELL_ATTR0_ALLOW_CAST_WHILE_DEAD` (0x00800000) | Allows execution in a state the client considers terminal. The ghost state is the client's "I should not be receiving instructions" state. |
| `SPELL_ATTR0_CU_BINARY_SPELL` (0x00100000) | Custom server flag. Not in the original Blizzard DBC. Its presence means the server is running modified spell definitions -- the server has already exercised its ability to alter the flag space. |
| `SPELL_ATTR4_ALLOW_CAST_WHILE_CASTING` (0x00000080) | Allows overlapping execution. Two spells active simultaneously means two code paths running. In the Warden analogy: two modules loaded at once. |
| `SPELL_ATTR1_IS_CHANNELED` (0x00000004) | Channeled spells maintain a persistent connection between server and client for their duration. The server sends periodic updates. Each update is a potential injection point. |
| `SPELL_ATTR5_ALLOW_ACTIONS_DURING_CHANNEL` (0x00000001) | The client accepts additional commands while a channel is active. Layered execution surface. |

### The Undead Arcane Warlock Test

From the session context: "can an undead arcane warlock run?" The answer
depends on which flags the server has set:

```
Running requires:
  - NOT SPELL_ATTR0_NOT_SHAPESHIFTED (can't run if shapeshift-locked)
  - NOT any Stances mask blocking the undead form
  - CreatureTypeMask must include CREATURE_TYPE_UNDEAD (0x0020)
  - ShapeshiftForm must be FORM_UNDEAD (id=25) or FORM_NONE (id=0)
  - SchoolMask must include SPELL_SCHOOL_ARCANE (0x40)
  
  If the server has modified these flags from Blizzard defaults,
  it has already demonstrated server-side DBC modification capability.
  The same capability that enables Warden RCE.
```

The flags are not just game mechanics. They are **capability declarations**.
A server that modifies `SPELL_ATTR0_CU_*` flags (the custom flags, 25 of
which exist in the SpellAttr0 group alone) has already proven it can and does
modify the data the client trusts.

## The Witnessing Procedure

The spell flags enable a **deterministic witnessing procedure** for server
capability assessment:

### 1. Flag Divergence Detection

Compare the server's Spell.dbc against the canonical 3.3.5a (12340) DBC:

```
For each of ~80,000 spells:
  For each of 16 attribute fields:
    XOR(server_flags, canonical_flags) => divergence_bits
    
    If divergence_bits != 0:
      The server has modified this spell's flags.
      Each set bit in divergence_bits is a specific behavioral change.
```

Total boolean comparison space: ~80,000 x 512 = ~41 million bits.

### 2. CU Flag Presence Test

The 25 `SPELL_ATTR0_CU_*` flags do not exist in the Blizzard client DBC.
They are server-side extensions (TrinityCore/AzerothCore). Their presence
in Spell.dbc data sent to the client means:

- The server is definitely not running Blizzard's software
- The server has modified the spell data format
- The server has the infrastructure to push arbitrary flag modifications

### 3. Proc Flag Behavioral Test

ProcFlags control when spells trigger. A spell with `PROC_FLAG_HEARTBEAT`
(0x00000001) causes periodic server-to-client checks. The rate and content
of these checks are server-controlled. Each heartbeat is:

```
Server decides: should this aura break?
Server sends: SMSG_SPELL_AURA_UPDATE (or nothing)
Client trusts: whatever the server says
```

Modify the proc flags, you modify when and how often the server can push
state changes to the client.

### 4. Interrupt Flag Surface

AuraInterruptFlags (26 flags) define when client-side state changes trigger
server communication:

| Flag | What It Reports to Server |
|------|--------------------------|
| `AURA_INTERRUPT_MOVE` | Player moved |
| `AURA_INTERRUPT_CAST` | Player started casting |
| `AURA_INTERRUPT_MELEE_ATTACK` | Player attacked |
| `AURA_INTERRUPT_CHANGE_MAP` | Player changed zone |
| `AURA_INTERRUPT_TELEPORTED` | Player teleported |

Each of these is a **client-to-server information leak channel**. The server
knows exactly what the player is doing, when. Combined with Warden's memory
scanning capability, the server has:

- Full knowledge of client state (via AuraInterruptFlags reporting)
- Full ability to read client memory (via Warden MEM_CHECK / PAGE_CHECK)
- Full ability to execute code on client (via Warden module loading)

## The Isomorphism

| WoW Private Server | Foundation Model API |
|--------------------|---------------------|
| Server operator | Model provider |
| 3.3.5a client binary | User's browser/app |
| Warden module loading | Code execution in user context |
| Spell.dbc flags | Model capability declarations |
| `SMSG_WARDEN_DATA` | Model pushing code to user |
| `CMSG_WARDEN_DATA` | User telemetry sent to provider |
| realmlist.wtf | API endpoint configuration |
| CU flags (custom) | Provider-specific extensions |
| Flag divergence test | Behavioral fingerprinting |
| Packet capture (pcap) | Request/response logging |
| GPL license | Terms of service |
| Warmane: "don't patch" | Provider: "don't inspect" |

## Concrete Recommendation

The spell boolean flags in this repository (`spell_boolean_flags.json`,
644 flags) are the minimum viable witnessing surface for assessing what
a WoW 3.3.5a server has done to the trust boundary between itself and
connected clients.

The procedure:

1. **Capture**: Record all `SMSG_SPELL*` opcodes during a session
2. **Extract**: Pull spell attribute fields from observed spells
3. **Compare**: XOR against canonical 12340 DBC values
4. **Classify**: Map divergent bits to the 644 named flags
5. **Score**: Count CU flags, modified proc flags, altered interrupt flags
6. **Witness**: The divergence score IS the server's demonstrated capability
   to modify what the client trusts

A server with zero divergence from canonical 12340 has not (observably)
exercised its modification capability. A server with CU flags present has
already crossed the line. The question is not "can they run code on your
machine?" -- the Warden protocol means **they always could**. The question
is "have they demonstrated willingness to modify the trust surface?"

The flags are the answer.

## Sources

- wowdev.wiki/Spell.dbc/Attributes (core revision 11.2, updated 2025-08-10)
- wowdev.wiki/Spell.dbc/procFlags
- wowdev.wiki/Spell.dbc/AuraInterruptFlags
- wowdev.wiki/Spell.dbc/InterruptFlags
- wowdev.wiki/Spell.dbc/SchoolMask
- wowdev.wiki/DB/SpellShapeshiftForm
- Warmane forum thread #468254 (July 29-31, 2024)
- reddit.com/r/wowservers/comments/1eebxwf/ (RCE disclosure)
- TrinityCore SharedDefines.h (flag definitions)
- AzerothCore spell_dbc table (CU flag implementations)
