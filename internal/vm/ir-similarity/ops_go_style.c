// ops_go_style.c — C translation of boxxy's Go SplitMix64/colorAt/seedFromName
// Exact same constants and bit-manipulation as vm.go for LLVM IR comparison.
#include <stdint.h>
#include <math.h>

// Gay.jl SplitMix64 bijection — identical constants to Go version
uint64_t splitmix64_go(uint64_t x) {
    x += 0x9e3779b97f4a7c15ULL;
    x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9ULL;
    x = (x ^ (x >> 27)) * 0x94d049bb133111ebULL;
    return x ^ (x >> 31);
}

// colorAt — same half-open arithmetic as Go (÷65536 not ÷65535)
void colorAt_go(uint64_t seed, uint64_t index,
                double *h, double *s, double *l) {
    uint64_t mixed = splitmix64_go(seed ^ index);
    *h = (double)(mixed & 0xFFFF) / 65536.0 * 360.0;
    *s = 0.5 + (double)((mixed >> 16) & 0xFFFF) / 65536.0 * 0.5;
    *l = 0.4 + (double)((mixed >> 32) & 0xFFFF) / 65536.0 * 0.2;
}

// FNV-1a — matches Go's hash/fnv New64a
uint64_t seedFromName_go(const char *name, int len) {
    uint64_t h = 14695981039346656037ULL; // FNV offset basis
    for (int i = 0; i < len; i++) {
        h ^= (uint64_t)(unsigned char)name[i];
        h *= 1099511628211ULL; // FNV prime
    }
    return h;
}

// clampHue — NaN/Inf-safe, half-open [0,360)
double clampHue_go(double h) {
    if (isnan(h) || isinf(h)) return 0.0;
    h = fmod(h, 360.0);
    if (h < 0.0) h += 360.0;
    if (h >= 360.0) h = 0.0;
    return h;
}

// clamp01 — NaN/Inf-safe [0,1]
double clamp01_go(double v) {
    if (isnan(v) || isinf(v)) {
        return (isinf(v) && v > 0) ? 1.0 : 0.0;
    }
    if (v < 0.0) return 0.0;
    if (v > 1.0) return 1.0;
    return v;
}
