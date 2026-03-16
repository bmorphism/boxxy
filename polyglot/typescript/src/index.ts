#!/usr/bin/env node
/**
 * Cat-clad anti-bullshit MCP server.
 *
 * Implements three tools for epistemological verification:
 *   1. analyze_claim  - structural verification of a claim
 *   2. validate_sources - source extraction + classification
 *   3. check_manipulation - manipulation pattern detection
 *
 * Uses @modelcontextprotocol/sdk with Server + StdioServerTransport.
 *
 * GF(3) role assignment:
 *   Claims  = 1 (Generator)  -- assertions create structure
 *   Sources = 2 (Verifier)   -- evidence checks claims
 *   Witness = 0 (Coordinator) -- attestation mediates
 *   Conservation: sum of trits must be 0 (mod 3)
 */

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";

import {
  analyzeClaim,
  validateSources,
  detectManipulation,
  Frameworks,
  type Framework,
} from "./catclad.js";

// ============================================================================
// Framework validation -- const-derived guard
// ============================================================================

const VALID_FRAMEWORKS = Object.freeze(
  Object.values(Frameworks),
) as ReadonlyArray<Framework>;

function isFramework(s: string): s is Framework {
  return (VALID_FRAMEWORKS as ReadonlyArray<string>).includes(s);
}

// ============================================================================
// Server setup
// ============================================================================

const server = new Server(
  {
    name: "anti-bullshit-catclad",
    version: "0.1.0",
  },
  {
    capabilities: {
      tools: {},
    },
  },
);

// ============================================================================
// Tool definitions
// ============================================================================

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [
    {
      name: "analyze_claim",
      description:
        "Analyze a claim using cat-clad epistemological verification. " +
        "Parses the text into a categorical structure with Claims (generators), " +
        "Sources (verifiers), Witnesses (coordinators), Derivations (morphisms), " +
        "and Cocycles (sheaf obstructions). Returns GF(3) balance and confidence.",
      inputSchema: {
        type: "object" as const,
        properties: {
          text: {
            type: "string",
            description: "The claim text to analyze",
          },
          framework: {
            type: "string",
            enum: ["empirical", "responsible", "harmonic", "pluralistic"],
            description:
              "Epistemological framework (default: pluralistic). " +
              "empirical boosts academic sources, responsible weights community impact, " +
              "harmonic rewards multi-source convergence, pluralistic uses raw structure.",
          },
        },
        required: ["text"],
      },
    },
    {
      name: "validate_sources",
      description:
        "Extract and classify sources from text as cat-clad morphisms. " +
        "Identifies academic references, authority citations, URLs, and anecdotal evidence. " +
        "Each source gets a GF(3) trit (verifier=2) and derivation strength.",
      inputSchema: {
        type: "object" as const,
        properties: {
          text: {
            type: "string",
            description: "Text containing claims and citations to validate",
          },
          framework: {
            type: "string",
            enum: ["empirical", "responsible", "harmonic", "pluralistic"],
            description: "Epistemological framework (default: pluralistic)",
          },
        },
        required: ["text"],
      },
    },
    {
      name: "check_manipulation",
      description:
        "Detect manipulation patterns in text. Checks for 10 patterns: " +
        "emotional_fear, urgency, false_consensus, appeal_authority, " +
        "artificial_scarcity, social_pressure, loaded_language, " +
        "false_dichotomy, circular_reasoning, ad_hominem. " +
        "Each match includes the evidence substring and severity (0-1).",
      inputSchema: {
        type: "object" as const,
        properties: {
          text: {
            type: "string",
            description: "Text to check for manipulation patterns",
          },
        },
        required: ["text"],
      },
    },
  ],
}));

// ============================================================================
// Tool handlers
// ============================================================================

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  switch (name) {
    case "analyze_claim": {
      const text = args?.text as string;
      if (!text) {
        return {
          content: [
            { type: "text", text: "Error: 'text' parameter is required" },
          ],
          isError: true,
        };
      }

      const framework =
        typeof args?.framework === "string" && isFramework(args.framework)
          ? args.framework
          : ("pluralistic" as const satisfies Framework);

      const world = analyzeClaim(text, framework);
      const { h1, cocycles } = world.sheafConsistency();
      const { balanced, counts } = world.gf3Balance();

      const result = {
        ...world.toJSON(),
        sheaf: { h1, cocycles },
        gf3: { balanced, counts },
        summary: {
          claimCount: world.claims.size,
          sourceCount: world.sources.size,
          witnessCount: world.witnesses.size,
          derivationCount: world.derivations.length,
          cocycleCount: world.cocycles.length,
          framework,
          confidence: Array.from(world.claims.values())[0]?.confidence ?? 0,
          isConsistent: h1 === 0,
          isBalanced: balanced,
        },
      } satisfies Record<string, unknown>;

      return {
        content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
      };
    }

    case "validate_sources": {
      const text = args?.text as string;
      if (!text) {
        return {
          content: [
            { type: "text", text: "Error: 'text' parameter is required" },
          ],
          isError: true,
        };
      }

      const framework =
        typeof args?.framework === "string" && isFramework(args.framework)
          ? args.framework
          : ("pluralistic" as const satisfies Framework);

      const world = validateSources(text, framework);

      const sourcesArray = Array.from(world.sources.values());
      const witnessesArray = Array.from(world.witnesses.values());
      const { balanced, counts } = world.gf3Balance();

      // Group sources by kind using Object.groupBy (ES2024)
      const byKindGrouped = Object.groupBy(sourcesArray, (s) => s.kind);
      const byKind: Record<string, number> = {};
      for (const [kind, group] of Object.entries(byKindGrouped)) {
        if (group) byKind[kind] = group.length;
      }

      const result = {
        sources: sourcesArray,
        witnesses: witnessesArray,
        derivations: world.derivations,
        gf3: { balanced, counts },
        summary: {
          totalSources: sourcesArray.length,
          byKind,
          averageStrength:
            world.derivations.length > 0
              ? world.derivations.reduce((sum, d) => sum + d.strength, 0) /
                world.derivations.length
              : 0,
        },
      } satisfies Record<string, unknown>;

      return {
        content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
      };
    }

    case "check_manipulation": {
      const text = args?.text as string;
      if (!text) {
        return {
          content: [
            { type: "text", text: "Error: 'text' parameter is required" },
          ],
          isError: true,
        };
      }

      const patterns = detectManipulation(text);
      const maxSeverity =
        patterns.length > 0
          ? Math.max(...patterns.map((p) => p.severity))
          : 0;

      const result = {
        patterns,
        summary: {
          totalPatterns: patterns.length,
          maxSeverity,
          isManipulative: patterns.length > 0,
          riskLevel:
            maxSeverity >= 0.8
              ? "high"
              : maxSeverity >= 0.5
                ? "medium"
                : maxSeverity > 0
                  ? "low"
                  : "none",
          byKind: patterns.reduce(
            (acc, p) => {
              acc[p.kind] = (acc[p.kind] || 0) + 1;
              return acc;
            },
            {} as Record<string, number>,
          ),
        },
      } satisfies Record<string, unknown>;

      return {
        content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
      };
    }

    default:
      return {
        content: [{ type: "text", text: `Unknown tool: ${name}` }],
        isError: true,
      };
  }
});

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("anti-bullshit-catclad MCP server running on stdio");
}

main().catch((error: unknown) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
