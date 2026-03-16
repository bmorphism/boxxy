//! Cat-clad anti-bullshit MCP server (JSON-RPC over stdio).
//!
//! Implements the Model Context Protocol by reading JSON-RPC 2.0 messages
//! from stdin and writing responses to stdout.  Registers three tools:
//!
//!   - analyze_claim:      parse text into a cat-clad ACSet, check sheaf consistency
//!   - validate_sources:   extract and score sources + GF(3) balance
//!   - check_manipulation: detect rhetorical manipulation patterns
//!
//! Transport: stdio (line-delimited JSON-RPC).

mod catclad;

use catclad::{analyze_claim, detect_manipulation, gf3_balance, sheaf_consistency, ClaimWorld};
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::io::{self, BufRead, Write};

// ---------------------------------------------------------------------------
// MCP protocol constants
// ---------------------------------------------------------------------------

const SERVER_NAME: &str = "catclad-anti-bullshit";
const SERVER_VERSION: &str = "0.1.0";
const PROTOCOL_VERSION: &str = "2024-11-05";

// ---------------------------------------------------------------------------
// Tool descriptors
// ---------------------------------------------------------------------------

fn tool_definitions() -> Value {
    json!([
        {
            "name": "analyze_claim",
            "description": "Analyze a textual claim using cat-clad epistemological verification. Parses text into an ACSet with claims, sources, witnesses, derivations; checks sheaf consistency (H^1) and GF(3) conservation.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "The claim text to analyze"
                    },
                    "framework": {
                        "type": "string",
                        "description": "Epistemological framework: empirical, responsible, harmonic, or pluralistic",
                        "enum": ["empirical", "responsible", "harmonic", "pluralistic"],
                        "default": "pluralistic"
                    }
                },
                "required": ["text"]
            }
        },
        {
            "name": "validate_sources",
            "description": "Extract and validate sources from a claim. Returns source classifications, derivation strengths, witness attestations, and GF(3) balance status.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "The text to extract and validate sources from"
                    }
                },
                "required": ["text"]
            }
        },
        {
            "name": "check_manipulation",
            "description": "Detect rhetorical manipulation patterns in text. Checks for emotional appeals, false consensus, urgency, loaded language, ad hominem, circular reasoning, and more.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "The text to check for manipulation patterns"
                    }
                },
                "required": ["text"]
            }
        }
    ])
}

// ---------------------------------------------------------------------------
// Tool execution
// ---------------------------------------------------------------------------

fn execute_analyze_claim(args: &Value) -> Value {
    let text = args
        .get("text")
        .and_then(|v| v.as_str())
        .unwrap_or("");
    let framework = args
        .get("framework")
        .and_then(|v| v.as_str())
        .unwrap_or("pluralistic");

    let world = analyze_claim(text, framework);
    let (h1, cocycles) = sheaf_consistency(&world);
    let (balanced, trit_counts) = gf3_balance(&world);

    format_analysis_result(&world, h1, cocycles, balanced, &trit_counts)
}

fn execute_validate_sources(args: &Value) -> Value {
    let text = args
        .get("text")
        .and_then(|v| v.as_str())
        .unwrap_or("");

    let world = analyze_claim(text, "pluralistic");
    let (balanced, trit_counts) = gf3_balance(&world);

    let sources: Vec<Value> = world
        .sources
        .values()
        .map(|s| {
            json!({
                "id": s.id,
                "citation": s.citation,
                "kind": s.kind.as_str(),
                "trit": format!("{}", s.trit),
            })
        })
        .collect();

    let derivations: Vec<Value> = world
        .derivations
        .iter()
        .map(|d| {
            json!({
                "source_id": d.source_id,
                "claim_id": d.claim_id,
                "kind": d.kind.as_str(),
                "strength": d.strength,
            })
        })
        .collect();

    let witnesses: Vec<Value> = world
        .witnesses
        .values()
        .map(|w| {
            json!({
                "id": w.id,
                "name": w.name,
                "role": w.role.as_str(),
                "weight": w.weight,
                "trit": format!("{}", w.trit),
            })
        })
        .collect();

    json!({
        "source_count": sources.len(),
        "sources": sources,
        "derivations": derivations,
        "witnesses": witnesses,
        "gf3_balanced": balanced,
        "trit_counts": trit_counts,
    })
}

fn execute_check_manipulation(args: &Value) -> Value {
    let text = args
        .get("text")
        .and_then(|v| v.as_str())
        .unwrap_or("");

    let patterns = detect_manipulation(text);
    let total_severity: f64 = patterns.iter().map(|p| p.severity).sum();
    let max_severity: f64 = patterns
        .iter()
        .map(|p| p.severity)
        .fold(0.0_f64, f64::max);

    let pattern_values: Vec<Value> = patterns
        .iter()
        .map(|p| {
            json!({
                "kind": p.kind,
                "evidence": p.evidence,
                "severity": p.severity,
            })
        })
        .collect();

    let risk_level = if max_severity >= 0.8 {
        "high"
    } else if max_severity >= 0.5 {
        "medium"
    } else if !patterns.is_empty() {
        "low"
    } else {
        "none"
    };

    json!({
        "manipulation_detected": !patterns.is_empty(),
        "pattern_count": patterns.len(),
        "patterns": pattern_values,
        "total_severity": total_severity,
        "max_severity": max_severity,
        "risk_level": risk_level,
    })
}

fn format_analysis_result(
    world: &ClaimWorld,
    h1: usize,
    cocycles: &[catclad::Cocycle],
    balanced: bool,
    trit_counts: &BTreeMap<String, usize>,
) -> Value {
    let claims: Vec<Value> = world
        .claims
        .values()
        .map(|c| {
            json!({
                "id": c.id,
                "text": c.text,
                "trit": format!("{}", c.trit),
                "confidence": c.confidence,
                "framework": c.framework,
                "hash": c.hash,
            })
        })
        .collect();

    let cocycle_values: Vec<Value> = cocycles
        .iter()
        .map(|c| {
            json!({
                "claim_a": c.claim_a,
                "claim_b": c.claim_b,
                "kind": c.kind.as_str(),
                "severity": c.severity,
            })
        })
        .collect();

    json!({
        "claims": claims,
        "source_count": world.sources.len(),
        "witness_count": world.witnesses.len(),
        "derivation_count": world.derivations.len(),
        "sheaf_consistency": {
            "h1_dimension": h1,
            "consistent": h1 == 0,
            "cocycles": cocycle_values,
        },
        "gf3_balance": {
            "balanced": balanced,
            "trit_counts": trit_counts,
        },
    })
}

// ---------------------------------------------------------------------------
// JSON-RPC dispatch
// ---------------------------------------------------------------------------

fn handle_request(req: &Value) -> Option<Value> {
    let id = req.get("id");
    let method = req.get("method").and_then(|m| m.as_str()).unwrap_or("");
    let params = req.get("params").cloned().unwrap_or(json!({}));

    match method {
        // ---- MCP lifecycle ----
        "initialize" => {
            let result = json!({
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {
                    "tools": {
                        "listChanged": false
                    }
                },
                "serverInfo": {
                    "name": SERVER_NAME,
                    "version": SERVER_VERSION,
                }
            });
            Some(jsonrpc_response(id, result))
        }

        // Notifications (no response)
        "notifications/initialized" | "notifications/cancelled" => None,

        "ping" => Some(jsonrpc_response(id, json!({}))),

        "tools/list" => {
            let result = json!({
                "tools": tool_definitions(),
            });
            Some(jsonrpc_response(id, result))
        }

        "tools/call" => {
            let tool_name = params
                .get("name")
                .and_then(|n| n.as_str())
                .unwrap_or("");
            let arguments = params
                .get("arguments")
                .cloned()
                .unwrap_or(json!({}));

            let tool_result = match tool_name {
                "analyze_claim" => execute_analyze_claim(&arguments),
                "validate_sources" => execute_validate_sources(&arguments),
                "check_manipulation" => execute_check_manipulation(&arguments),
                unknown => {
                    return Some(jsonrpc_error(
                        id,
                        -32602,
                        &format!("Unknown tool: {}", unknown),
                    ));
                }
            };

            let content_text =
                serde_json::to_string_pretty(&tool_result).unwrap_or_else(|_| "{}".to_string());

            let result = json!({
                "content": [
                    {
                        "type": "text",
                        "text": content_text,
                    }
                ],
                "isError": false,
            });
            Some(jsonrpc_response(id, result))
        }

        _ => Some(jsonrpc_error(
            id,
            -32601,
            &format!("Method not found: {}", method),
        )),
    }
}

fn jsonrpc_response(id: Option<&Value>, result: Value) -> Value {
    json!({
        "jsonrpc": "2.0",
        "id": id.cloned().unwrap_or(Value::Null),
        "result": result,
    })
}

fn jsonrpc_error(id: Option<&Value>, code: i64, message: &str) -> Value {
    json!({
        "jsonrpc": "2.0",
        "id": id.cloned().unwrap_or(Value::Null),
        "error": {
            "code": code,
            "message": message,
        },
    })
}

// ---------------------------------------------------------------------------
// Main event loop
// ---------------------------------------------------------------------------

fn main() {
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut stdout_lock = stdout.lock();

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };

        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }

        let request: Value = match serde_json::from_str(trimmed) {
            Ok(v) => v,
            Err(e) => {
                let err = jsonrpc_error(None, -32700, &format!("Parse error: {}", e));
                let _ = writeln!(stdout_lock, "{}", serde_json::to_string(&err).unwrap());
                let _ = stdout_lock.flush();
                continue;
            }
        };

        if let Some(response) = handle_request(&request) {
            let _ = writeln!(
                stdout_lock,
                "{}",
                serde_json::to_string(&response).unwrap()
            );
            let _ = stdout_lock.flush();
        }
    }
}

// ---------------------------------------------------------------------------
// Integration tests for the MCP server protocol
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_initialize() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": { "name": "test", "version": "0.1" }
            }
        });

        let resp = handle_request(&req).expect("should return response");
        assert_eq!(resp["jsonrpc"], "2.0");
        assert_eq!(resp["id"], 1);
        let result = &resp["result"];
        assert_eq!(result["protocolVersion"], PROTOCOL_VERSION);
        assert_eq!(result["serverInfo"]["name"], SERVER_NAME);
    }

    #[test]
    fn test_tools_list() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list",
            "params": {}
        });

        let resp = handle_request(&req).expect("should return response");
        let tools = resp["result"]["tools"].as_array().expect("tools array");
        assert_eq!(tools.len(), 3);

        let names: Vec<&str> = tools
            .iter()
            .map(|t| t["name"].as_str().unwrap())
            .collect();
        assert!(names.contains(&"analyze_claim"));
        assert!(names.contains(&"validate_sources"));
        assert!(names.contains(&"check_manipulation"));
    }

    #[test]
    fn test_tools_call_analyze_claim() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/call",
            "params": {
                "name": "analyze_claim",
                "arguments": {
                    "text": "According to Dr. Smith, research from Harvard shows positive results",
                    "framework": "empirical"
                }
            }
        });

        let resp = handle_request(&req).expect("should return response");
        assert!(resp.get("error").is_none(), "should not have error");
        let content = &resp["result"]["content"];
        assert!(content.is_array());
        assert_eq!(content[0]["type"], "text");

        // Parse the inner text as JSON and verify structure
        let inner: Value =
            serde_json::from_str(content[0]["text"].as_str().unwrap()).expect("valid JSON");
        assert!(inner.get("claims").is_some());
        assert!(inner.get("sheaf_consistency").is_some());
        assert!(inner.get("gf3_balance").is_some());
    }

    #[test]
    fn test_tools_call_validate_sources() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 4,
            "method": "tools/call",
            "params": {
                "name": "validate_sources",
                "arguments": {
                    "text": "A study by MIT published in Nature and https://example.com/data"
                }
            }
        });

        let resp = handle_request(&req).expect("should return response");
        assert!(resp.get("error").is_none());
        let inner: Value = serde_json::from_str(
            resp["result"]["content"][0]["text"]
                .as_str()
                .unwrap(),
        )
        .expect("valid JSON");
        assert!(inner["source_count"].as_u64().unwrap() > 0);
        assert!(inner.get("gf3_balanced").is_some());
    }

    #[test]
    fn test_tools_call_check_manipulation() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 5,
            "method": "tools/call",
            "params": {
                "name": "check_manipulation",
                "arguments": {
                    "text": "Act now! Everyone knows this exclusive deal expires soon."
                }
            }
        });

        let resp = handle_request(&req).expect("should return response");
        assert!(resp.get("error").is_none());
        let inner: Value = serde_json::from_str(
            resp["result"]["content"][0]["text"]
                .as_str()
                .unwrap(),
        )
        .expect("valid JSON");
        assert!(inner["manipulation_detected"].as_bool().unwrap());
        assert!(inner["pattern_count"].as_u64().unwrap() > 0);
        assert!(inner["risk_level"].as_str().unwrap() != "none");
    }

    #[test]
    fn test_tools_call_clean_text() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 6,
            "method": "tools/call",
            "params": {
                "name": "check_manipulation",
                "arguments": {
                    "text": "The temperature is 72 degrees with clear skies."
                }
            }
        });

        let resp = handle_request(&req).expect("should return response");
        let inner: Value = serde_json::from_str(
            resp["result"]["content"][0]["text"]
                .as_str()
                .unwrap(),
        )
        .expect("valid JSON");
        assert!(!inner["manipulation_detected"].as_bool().unwrap());
        assert_eq!(inner["risk_level"], "none");
    }

    #[test]
    fn test_unknown_tool() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 7,
            "method": "tools/call",
            "params": {
                "name": "nonexistent_tool",
                "arguments": {}
            }
        });

        let resp = handle_request(&req).expect("should return response");
        assert!(resp.get("error").is_some());
        assert_eq!(resp["error"]["code"], -32602);
    }

    #[test]
    fn test_unknown_method() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 8,
            "method": "nonexistent/method",
            "params": {}
        });

        let resp = handle_request(&req).expect("should return response");
        assert!(resp.get("error").is_some());
        assert_eq!(resp["error"]["code"], -32601);
    }

    #[test]
    fn test_notification_returns_none() {
        let req = json!({
            "jsonrpc": "2.0",
            "method": "notifications/initialized"
        });

        let resp = handle_request(&req);
        assert!(resp.is_none(), "notifications should not produce a response");
    }

    #[test]
    fn test_ping() {
        let req = json!({
            "jsonrpc": "2.0",
            "id": 9,
            "method": "ping",
            "params": {}
        });

        let resp = handle_request(&req).expect("should return response");
        assert_eq!(resp["jsonrpc"], "2.0");
        assert!(resp.get("error").is_none());
    }
}
