// ops_java_style.c — Java/SplittableRandom-style implementation of the same ops.
// Different variable names and structure, but operationally identical arithmetic.
#include <stdint.h>
#include <math.h>

// Java SplittableRandom.mix64 — same constants as Gay.jl/Go, different var names
static uint64_t mix64(uint64_t z) {
    z += 0x9e3779b97f4a7c15ULL;           // GOLDEN
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ULL; // STAFFORD_MIX1
    z = (z ^ (z >> 27)) * 0x94d049bb133111ebULL;  // STAFFORD_MIX2
    return z ^ (z >> 31);
}

// Deterministic HSL from (gamma, position) — same math, different names
struct HSLColor {
    double hue;
    double saturation;
    double lightness;
};

struct HSLColor deterministicHSL(uint64_t gamma, uint64_t position) {
    uint64_t hash = mix64(gamma ^ position);
    struct HSLColor c;
    c.hue        = (double)(hash & 0xFFFF) / 65536.0 * 360.0;
    c.saturation = 0.5 + (double)((hash >> 16) & 0xFFFF) / 65536.0 * 0.5;
    c.lightness  = 0.4 + (double)((hash >> 32) & 0xFFFF) / 65536.0 * 0.2;
    return c;
}

// FNV-1a hash — same algorithm, loop-style differs
uint64_t fnv1a_hash(const uint8_t *data, uint64_t length) {
    uint64_t result = 14695981039346656037ULL;
    for (uint64_t idx = 0; idx < length; idx++) {
        result ^= (uint64_t)data[idx];
        result *= 1099511628211ULL;
    }
    return result;
}

// Normalize hue to [0, 360) — same logic, different guard order
double normalizeHue(double h) {
    if (isinf(h) || isnan(h)) return 0.0;
    h = fmod(h, 360.0);
    if (h < 0.0) h += 360.0;
    if (h >= 360.0) h = 0.0;
    return h;
}

// Saturate to [0, 1] — rewritten with ternary
double saturate(double v) {
    if (isnan(v)) return 0.0;
    if (isinf(v)) return (v > 0.0) ? 1.0 : 0.0;
    return v < 0.0 ? 0.0 : (v > 1.0 ? 1.0 : v);
}
