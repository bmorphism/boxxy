# Darcs in Boxxy

Boxxy uses **darcs** alongside git (and pijul) as a multi-VCS repository.
The `_darcs/` directory sits alongside `.git/` and `.pijul/`.

**darcs version**: 2.18.5
**binary**: `~/.nix-profile/bin/darcs`

## Darcs Patch Theory and GF(3) Triadic System

Darcs is built on a *theory of patches* where patches are first-class
mathematical objects with well-defined commutation laws. This resonates
deeply with boxxy's GF(3) triadic architecture:

### The Commutation Analogy

In darcs, two patches `p` and `q` can commute if applying `p` then `q`
yields the same state as applying `q'` then `p'` (the commuted versions).
The commutation relation is:

    p ; q  <-->  q' ; p'

This is structurally analogous to GF(3) addition's commutativity:

    Add(a, b) = Add(b, a)  (mod 3)

### Patches as Balanced Triads

Darcs patches naturally partition into three roles that map onto boxxy's
GF(3) skill roles:

| Darcs Operation | GF(3) Role        | Trit |
|----------------|--------------------|------|
| `addfile`/`adddir` (creation) | Generator (+1) | 1 |
| `move`/`replace` (refactor)   | Coordinator (0) | 0 |
| `rmfile`/`rmdir` (removal)    | Verifier (-1)   | 2 |

The conservation law `sum(trits) = 0 (mod 3)` maps to the darcs invariant
that a well-formed patch sequence must yield a consistent repository state.
Creation and destruction patches must balance over time, with coordinative
patches maintaining equilibrium.

### Inverse Patches and GF(3) Negation

Every darcs patch has an inverse (its "unpatch"). This mirrors GF(3)'s
`Neg(a) = (3 - a) % 3`:

- Inverse of `addfile` is `rmfile` (Generator inverts to Verifier: Neg(1) = 2)
- Inverse of `rmfile` is `addfile` (Verifier inverts to Generator: Neg(2) = 1)
- Inverse of `replace` is `replace` with swapped args (Coordinator self-inverts: Neg(0) = 0)

### Conflict as Incomparability

When two darcs patches conflict (cannot commute), this is precisely the
*incomparability* condition from Lindstrom-Rabinowicz belief revision that
boxxy formalizes in `theories/AGM_Extensions.thy`. Conflict resolution in
darcs corresponds to *selection functions* that determinize the
indeterministic revision --- the Hedges/Vibesnipe pattern.

### Patch Dependencies as Capability Lattice

Darcs patch dependencies form a partial order (a lattice). This maps
onto boxxy's seL4 capability model: a patch can only be applied if its
dependencies are satisfied, just as a capability can only be invoked
if its parent capabilities exist in the CSpace.

## Daily Use Commands

### Recording Changes

```bash
# See what has changed (like git status + diff)
darcs whatsnew
darcs whatsnew -s          # summary only (like git status)

# Record all changes with a message
darcs record -a -m "description"

# Interactive record (select hunks, like git add -p)
darcs record

# Record only changes to specific files
darcs record cmd/boxxy/main.go
```

### Viewing History

```bash
# Show recent patches
darcs log --last=5
darcs log --last=1 -v      # verbose (shows changes)

# Show repo summary
darcs show repo

# Show what files are tracked
darcs show files

# Show current unrecorded changes
darcs diff
```

### Amending and Unrecording

```bash
# Amend the last patch (add more changes to it)
darcs amend

# Unrecord the last patch (keeps changes in working dir)
darcs unrecord --last=1

# Obliterate the last patch (removes changes entirely)
darcs obliterate --last=1
```

### Cherry-picking and Reordering

```bash
# Reorder patches interactively
darcs reorder

# Pull specific patches from another repo
darcs pull --interactive /path/to/other/repo
```

### Adding and Removing Files

```bash
# Add new files/directories
darcs add newfile.go
darcs add -r newdir/
darcs add --boring go.mod    # force-add files matching boring patterns

# Remove tracked files
darcs remove oldfile.go
```

## Setting Up a Remote Darcs Repository

### Via SSH

```bash
# Create a bare repo on a remote server
ssh user@server "darcs init /srv/darcs/boxxy"

# Push patches to remote
darcs push user@server:/srv/darcs/boxxy

# Pull patches from remote
darcs pull user@server:/srv/darcs/boxxy

# Set a default remote (stored in _darcs/prefs/defaultrepo)
echo "user@server:/srv/darcs/boxxy" > _darcs/prefs/defaultrepo
darcs push   # now pushes to default
```

### Via darcs hub (hub.darcs.net)

```bash
# 1. Create an account at https://hub.darcs.net
# 2. Create a repo via the web UI
# 3. Push:
darcs push me@hub.darcs.net:boxxy

# Or clone from hub:
darcs clone https://hub.darcs.net/me/boxxy
```

### Via HTTP (read-only)

```bash
# Serve locally for pulls
cd /Users/bob/i/boxxy
python3 -m http.server 8080

# Others can pull:
darcs pull http://yourhost:8080/
```

### Mirroring git <-> darcs

To keep git and darcs in sync, record in both after changes:

```bash
# After editing:
git add -A && git commit -m "change description"
darcs record -a -m "change description"
```

Or use a post-commit hook (`.git/hooks/post-commit`):

```bash
#!/bin/bash
MSG=$(git log -1 --format=%s)
~/.nix-profile/bin/darcs record -a -m "$MSG" --author "bmorphism <bmorphism@gmail.com>" 2>/dev/null || true
```

## Repository Layout

```
/Users/bob/i/boxxy/
  .git/              # git history
  .pijul/            # pijul history
  _darcs/            # darcs history
    inventories/     # patch metadata
    patches/         # actual patch data
    pristine.hashed/ # pristine tree (hashed)
    prefs/
      author         # "bmorphism <bmorphism@gmail.com>"
      defaultrepo    # (set when you push/pull)
      boring         # default boring patterns
  .darcsignore       # regex patterns for ignored files
  .ignore            # pijul ignore
```
