#!/usr/bin/env python3
"""Build cyclomatic complexity distance matrix for boxxy Go functions.
Outputs CSV distance matrix to stdout.
Also outputs function labels to stderr for reference.
"""
import subprocess
import re
import sys
import json
import numpy as np

def get_go_functions(repo_path):
    """Extract Go functions with cyclomatic complexity estimates."""
    # Find all .go files (excluding vendor, test files for cleaner signal)
    result = subprocess.run(
        ["find", repo_path, "-name", "*.go", "-not", "-path", "*/vendor/*"],
        capture_output=True, text=True
    )
    go_files = result.stdout.strip().split("\n")

    functions = []
    for filepath in go_files:
        if not filepath.strip():
            continue
        try:
            with open(filepath, "r") as f:
                content = f.read()
        except (IOError, UnicodeDecodeError):
            continue

        rel_path = filepath.replace(repo_path + "/", "")

        # Find function definitions and estimate complexity
        # Pattern: func (receiver) Name(params) {
        func_pattern = re.compile(
            r'^func\s+(?:\([^)]*\)\s+)?(\w+)\s*\([^)]*\)',
            re.MULTILINE
        )

        lines = content.split("\n")
        for match in func_pattern.finditer(content):
            func_name = match.group(1)
            start_line = content[:match.start()].count("\n")

            # Find function body (count braces)
            depth = 0
            func_lines = []
            started = False
            for i, line in enumerate(lines[start_line:], start=start_line):
                if "{" in line:
                    depth += line.count("{")
                    started = True
                if "}" in line:
                    depth -= line.count("}")
                if started:
                    func_lines.append(line)
                if started and depth <= 0:
                    break

            body = "\n".join(func_lines)

            # Estimate cyclomatic complexity:
            # CC = 1 + decisions (if, else if, case, for, &&, ||, select)
            cc = 1
            cc += len(re.findall(r'\bif\b', body))
            cc += len(re.findall(r'\belse\s+if\b', body))
            cc += len(re.findall(r'\bcase\b', body))
            cc += len(re.findall(r'\bfor\b', body))
            cc += len(re.findall(r'&&', body))
            cc += len(re.findall(r'\|\|', body))
            cc += len(re.findall(r'\bselect\b', body))
            cc += len(re.findall(r'\bswitch\b', body))

            loc = len(func_lines)

            # Feature vector: [cyclomatic_complexity, lines_of_code, nesting_depth]
            max_depth = 0
            d = 0
            for line in func_lines:
                d += line.count("{") - line.count("}")
                max_depth = max(max_depth, d)

            functions.append({
                "name": f"{rel_path}:{func_name}",
                "cc": cc,
                "loc": loc,
                "depth": max_depth,
                "file": rel_path,
            })

    return functions

def build_distance_matrix(functions):
    """Build distance matrix from function complexity features.
    Distance = weighted L1 of (CC, LOC, depth) features."""
    n = len(functions)
    dm = np.zeros((n, n))

    for i in range(n):
        for j in range(i):
            # Weighted L1 distance on normalized features
            d_cc = abs(functions[i]["cc"] - functions[j]["cc"])
            d_loc = abs(functions[i]["loc"] - functions[j]["loc"]) / 10.0  # scale LOC
            d_depth = abs(functions[i]["depth"] - functions[j]["depth"]) * 2.0

            dist = d_cc + d_loc + d_depth
            dm[i, j] = dm[j, i] = dist

    return dm

if __name__ == "__main__":
    repo = sys.argv[1] if len(sys.argv) > 1 else "/Users/bob/i/boxxy"

    funcs = get_go_functions(repo)

    # Filter to functions with CC > 1 (non-trivial) for cleaner diagram
    funcs = [f for f in funcs if f["cc"] > 1]

    # Sort by complexity descending for readability
    funcs.sort(key=lambda f: -f["cc"])

    # Print labels to stderr
    print(f"# {len(funcs)} functions", file=sys.stderr)
    labels = []
    for i, f in enumerate(funcs):
        label = f"{f['name']} (CC={f['cc']}, LOC={f['loc']}, depth={f['depth']})"
        print(f"  [{i}] {label}", file=sys.stderr)
        labels.append(label)

    dm = build_distance_matrix(funcs)

    # Save labels as JSON
    with open("/tmp/boxxy-func-labels.json", "w") as f:
        json.dump([{"idx": i, "name": funcs[i]["name"], "cc": funcs[i]["cc"],
                     "loc": funcs[i]["loc"], "depth": funcs[i]["depth"]}
                    for i in range(len(funcs))], f, indent=2)

    # Output CSV distance matrix
    np.savetxt(sys.stdout, dm, delimiter=",", fmt="%.4f")
