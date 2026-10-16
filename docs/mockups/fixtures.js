// Fictional design data. Public catalog metadata is attributed in README.md.
// Keep this serializable object portable to Go demo fixtures. No credentials.
globalThis.QUOTARY_FIXTURES = {
  "fictional": true,
  "clock": "2026-10-16T14:00:00Z",
  "timezone": "UTC",
  "notice": "All readings, plans, entitlement, model names, capabilities and history are illustrative. Only catalog IDs/display names come from the pinned public provider catalog. This is not a verified capability manifest.",
  "sources": [
    {
      "id": "claude:configured",
      "provider": "claude",
      "name": "Claude Code",
      "plan": "Max",
      "windows": [
        {
          "label": "5h",
          "left": 72,
          "reset": "2h 14m",
          "resetMinutes": 134,
          "group": null
        },
        {
          "label": "Weekly",
          "left": 38,
          "reset": "3d 05h",
          "resetMinutes": 4620,
          "group": null
        },
        {
          "label": "Sonnet",
          "left": 86,
          "reset": "3d 05h",
          "resetMinutes": 4620,
          "group": null
        }
      ],
      "state": "current",
      "age": "34s",
      "kind": "Subscription",
      "history": true,
      "pinned": true,
      "pinOrder": 0
    },
    {
      "id": "codex:configured",
      "provider": "codex",
      "name": "Codex",
      "plan": "Pro",
      "windows": [
        {
          "label": "5h",
          "left": 84,
          "reset": "3h 08m",
          "resetMinutes": 188,
          "group": null
        },
        {
          "label": "Weekly",
          "left": 61,
          "reset": "4d 11h",
          "resetMinutes": 6420,
          "group": null
        },
        {
          "label": "Review",
          "left": 95,
          "reset": "4d 11h",
          "resetMinutes": 6420,
          "group": null
        }
      ],
      "state": "current",
      "age": "33s",
      "kind": "Subscription",
      "history": true,
      "pinned": true,
      "pinOrder": 1
    },
    {
      "id": "muse:configured",
      "provider": "muse",
      "name": "Muse Code",
      "plan": "Power Usage",
      "windows": [
        {
          "label": "5h",
          "left": 63,
          "reset": "1h 46m",
          "resetMinutes": 106,
          "group": null
        },
        {
          "label": "Weekly",
          "left": 47,
          "reset": "2d 03h",
          "resetMinutes": 3060,
          "group": null
        }
      ],
      "state": "current",
      "age": "32s",
      "kind": "Subscription",
      "history": true,
      "pinned": true,
      "pinOrder": 2
    },
    {
      "id": "gemini:web",
      "provider": "gemini",
      "name": "Google · Gemini app",
      "plan": "AI Pro",
      "windows": [
        {
          "label": "5h",
          "left": 92,
          "reset": "4h 12m",
          "resetMinutes": 252,
          "group": null
        },
        {
          "label": "Weekly",
          "left": 69,
          "reset": "5d 01h",
          "resetMinutes": 7260,
          "group": null
        }
      ],
      "state": "current",
      "age": "31s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 3
    },
    {
      "id": "antigravity:configured",
      "provider": "antigravity",
      "name": "Google · Antigravity",
      "plan": "AI Pro",
      "windows": [
        {
          "label": "5h",
          "left": 18,
          "reset": "56m",
          "resetMinutes": 56,
          "group": "Gemini models"
        },
        {
          "label": "Weekly",
          "left": 26,
          "reset": "2d 09h",
          "resetMinutes": 3420,
          "group": "Gemini models"
        },
        {
          "label": "5h",
          "left": 67,
          "reset": "2h 30m",
          "resetMinutes": 150,
          "group": "Claude models"
        },
        {
          "label": "Weekly",
          "left": 54,
          "reset": "4d 02h",
          "resetMinutes": 5880,
          "group": "Claude models"
        }
      ],
      "state": "current",
      "age": "30s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 4
    },
    {
      "id": "cursor:configured",
      "provider": "cursor",
      "name": "Cursor",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 56,
          "reset": "12d",
          "resetMinutes": 17280,
          "group": null
        }
      ],
      "state": "current",
      "age": "29s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 5
    },
    {
      "id": "copilot:configured",
      "provider": "copilot",
      "name": "GitHub Copilot",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 81,
          "reset": "15d",
          "resetMinutes": 21600,
          "group": null
        }
      ],
      "state": "current",
      "age": "28s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 6
    },
    {
      "id": "windsurf:configured",
      "provider": "windsurf",
      "name": "Windsurf",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 44,
          "reset": "11d",
          "resetMinutes": 15840,
          "group": null
        }
      ],
      "state": "current",
      "age": "27s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 7
    },
    {
      "id": "kimi:configured",
      "provider": "kimi",
      "name": "Kimi Code",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 33,
          "reset": null,
          "resetMinutes": null,
          "group": null
        }
      ],
      "state": "current",
      "age": "26s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 8
    },
    {
      "id": "gemini:api",
      "provider": "gemini",
      "name": "Google · Gemini CLI",
      "plan": "AI Pro",
      "windows": [
        {
          "label": "Daily",
          "left": 78,
          "reset": "8h 10m",
          "resetMinutes": 490,
          "group": null
        }
      ],
      "state": "current",
      "age": "25s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 9
    },
    {
      "id": "kiro:configured",
      "provider": "kiro",
      "name": "Kiro",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 68,
          "reset": "13d",
          "resetMinutes": 18720,
          "group": null
        }
      ],
      "state": "current",
      "age": "24s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 10
    },
    {
      "id": "jetbrains:configured",
      "provider": "jetbrains",
      "name": "JetBrains AI",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 73,
          "reset": "10d",
          "resetMinutes": 14400,
          "group": null
        }
      ],
      "state": "current",
      "age": "23s",
      "kind": "Subscription",
      "history": false,
      "pinned": true,
      "pinOrder": 11
    },
    {
      "id": "zed:configured",
      "provider": "zed",
      "name": "Zed",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 59,
          "reset": "9d",
          "resetMinutes": 12960,
          "group": null
        }
      ],
      "state": "current",
      "age": "22s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 12
    },
    {
      "id": "warp:configured",
      "provider": "warp",
      "name": "Warp",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 12,
          "reset": "2d",
          "resetMinutes": 2880,
          "group": null
        }
      ],
      "state": "current",
      "age": "21s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 13
    },
    {
      "id": "augment:configured",
      "provider": "augment",
      "name": "Augment",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 47,
          "reset": "6d",
          "resetMinutes": 8640,
          "group": null
        }
      ],
      "state": "current",
      "age": "20s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 14
    },
    {
      "id": "kilo:configured",
      "provider": "kilo",
      "name": "Kilo",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 88,
          "reset": "17d",
          "resetMinutes": 24480,
          "group": null
        }
      ],
      "state": "current",
      "age": "19s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 15
    },
    {
      "id": "factory:configured",
      "provider": "factory",
      "name": "Droid",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 54,
          "reset": "8d",
          "resetMinutes": 11520,
          "group": null
        }
      ],
      "state": "current",
      "age": "18s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 16
    },
    {
      "id": "opencodego:configured",
      "provider": "opencodego",
      "name": "OpenCode Go",
      "plan": "Go",
      "windows": [
        {
          "label": "5h",
          "left": 91,
          "reset": "3h 20m",
          "resetMinutes": 200,
          "group": null
        },
        {
          "label": "Weekly",
          "left": 62,
          "reset": "4d",
          "resetMinutes": 5760,
          "group": null
        }
      ],
      "state": "current",
      "age": "17s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 17
    },
    {
      "id": "zai:configured",
      "provider": "zai",
      "name": "z.ai / GLM",
      "plan": "Coding Plan",
      "windows": [
        {
          "label": "5h",
          "left": 74,
          "reset": "2h 05m",
          "resetMinutes": 125,
          "group": null
        }
      ],
      "state": "current",
      "age": "16s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 18
    },
    {
      "id": "minimax:configured",
      "provider": "minimax",
      "name": "MiniMax",
      "plan": "Coding Plan",
      "windows": [
        {
          "label": "5h",
          "left": 82,
          "reset": "4h 15m",
          "resetMinutes": 255,
          "group": null
        }
      ],
      "state": "current",
      "age": "15s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 19
    },
    {
      "id": "perplexity:configured",
      "provider": "perplexity",
      "name": "Perplexity",
      "plan": "Pro",
      "windows": [
        {
          "label": "Daily",
          "left": 64,
          "reset": "8h",
          "resetMinutes": 480,
          "group": null
        }
      ],
      "state": "current",
      "age": "14s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 20
    },
    {
      "id": "qoder:configured",
      "provider": "qoder",
      "name": "Qoder",
      "plan": "Pro",
      "windows": [
        {
          "label": "Plan",
          "left": 43,
          "reset": "5d",
          "resetMinutes": 7200,
          "group": null
        }
      ],
      "state": "current",
      "age": "13s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 21
    },
    {
      "id": "clinepass:configured",
      "provider": "clinepass",
      "name": "ClinePass",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 77,
          "reset": "14d",
          "resetMinutes": 20160,
          "group": null
        }
      ],
      "state": "current",
      "age": "12s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 22
    },
    {
      "id": "devin:configured",
      "provider": "devin",
      "name": "Devin",
      "plan": "Subscription",
      "windows": [
        {
          "label": "Plan",
          "left": 51,
          "reset": "7d",
          "resetMinutes": 10080,
          "group": null
        }
      ],
      "state": "current",
      "age": "11s",
      "kind": "Subscription",
      "history": false,
      "pinned": false,
      "pinOrder": 23
    }
  ],
  "catalog": [
    {
      "provider": "codex",
      "enabled": true,
      "defaultEnabled": true,
      "displayName": "Codex"
    },
    {
      "provider": "openai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "OpenAI"
    },
    {
      "provider": "azureopenai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Azure OpenAI"
    },
    {
      "provider": "claude",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Claude"
    },
    {
      "provider": "clinepass",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ClinePass"
    },
    {
      "provider": "cursor",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Cursor"
    },
    {
      "provider": "opencode",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "OpenCode"
    },
    {
      "provider": "opencodego",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "OpenCode Go"
    },
    {
      "provider": "alibaba",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Alibaba"
    },
    {
      "provider": "alibabatokenplan",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Alibaba Token Plan"
    },
    {
      "provider": "qwencloud",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Qwen Cloud"
    },
    {
      "provider": "factory",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Droid"
    },
    {
      "provider": "fireworks",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Fireworks"
    },
    {
      "provider": "gemini",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Gemini"
    },
    {
      "provider": "antigravity",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Antigravity"
    },
    {
      "provider": "copilot",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Copilot"
    },
    {
      "provider": "devin",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Devin"
    },
    {
      "provider": "zai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "z.ai / GLM"
    },
    {
      "provider": "minimax",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "MiniMax"
    },
    {
      "provider": "manus",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Manus"
    },
    {
      "provider": "kimi",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Kimi Code"
    },
    {
      "provider": "kilo",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Kilo"
    },
    {
      "provider": "kiro",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Kiro"
    },
    {
      "provider": "vertexai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Vertex AI"
    },
    {
      "provider": "augment",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Augment"
    },
    {
      "provider": "jetbrains",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "JetBrains AI"
    },
    {
      "provider": "moonshot",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Moonshot / Kimi Open Platform"
    },
    {
      "provider": "amp",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Amp"
    },
    {
      "provider": "t3chat",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "T3 Chat"
    },
    {
      "provider": "langdock",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Langdock"
    },
    {
      "provider": "ollama",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Ollama"
    },
    {
      "provider": "synthetic",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Synthetic"
    },
    {
      "provider": "openrouter",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "OpenRouter"
    },
    {
      "provider": "elevenlabs",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ElevenLabs"
    },
    {
      "provider": "warp",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Warp"
    },
    {
      "provider": "windsurf",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Windsurf"
    },
    {
      "provider": "zed",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Zed"
    },
    {
      "provider": "perplexity",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Perplexity"
    },
    {
      "provider": "mimo",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Xiaomi MiMo"
    },
    {
      "provider": "doubao",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Doubao"
    },
    {
      "provider": "sakana",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Sakana AI"
    },
    {
      "provider": "abacus",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Abacus AI"
    },
    {
      "provider": "mistral",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Mistral"
    },
    {
      "provider": "deepseek",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "DeepSeek"
    },
    {
      "provider": "deepinfra",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "DeepInfra"
    },
    {
      "provider": "codebuff",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Codebuff"
    },
    {
      "provider": "venice",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Venice"
    },
    {
      "provider": "commandcode",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Command Code"
    },
    {
      "provider": "qoder",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Qoder"
    },
    {
      "provider": "stepfun",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "StepFun"
    },
    {
      "provider": "bedrock",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "AWS Bedrock"
    },
    {
      "provider": "grok",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Grok"
    },
    {
      "provider": "groq",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Groq"
    },
    {
      "provider": "llmproxy",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "LLM Proxy"
    },
    {
      "provider": "litellm",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "LiteLLM"
    },
    {
      "provider": "bifrost",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Bifrost"
    },
    {
      "provider": "aixy",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Aixy"
    },
    {
      "provider": "deepgram",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Deepgram"
    },
    {
      "provider": "poe",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Poe"
    },
    {
      "provider": "chutes",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Chutes"
    },
    {
      "provider": "neuralwatt",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Neuralwatt"
    },
    {
      "provider": "helmcode",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Helmcode"
    },
    {
      "provider": "clawrouter",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ClawRouter"
    },
    {
      "provider": "longcat",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "LongCat"
    },
    {
      "provider": "sub2api",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "sub2api"
    },
    {
      "provider": "wayfinder",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Wayfinder"
    },
    {
      "provider": "zenmux",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ZenMux"
    },
    {
      "provider": "aiand",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ai&"
    },
    {
      "provider": "zoommate",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "ZoomMate"
    },
    {
      "provider": "xai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "xAI"
    },
    {
      "provider": "xapi",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "X API"
    },
    {
      "provider": "notion",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Notion AI"
    },
    {
      "provider": "ibmbob",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "IBM Bob"
    },
    {
      "provider": "nous",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Nous Portal"
    },
    {
      "provider": "muse",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Muse Code"
    },
    {
      "provider": "coderabbit",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "CodeRabbit"
    },
    {
      "provider": "replicate",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Replicate"
    },
    {
      "provider": "huggingface",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Hugging Face"
    },
    {
      "provider": "raycast",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Raycast"
    },
    {
      "provider": "pi",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Pi"
    },
    {
      "provider": "v0",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "v0"
    },
    {
      "provider": "typesafe",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "TypeSafe"
    },
    {
      "provider": "hyper",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Charm Hyper"
    },
    {
      "provider": "gitkraken",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "GitKraken AI"
    },
    {
      "provider": "futureagi",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Future AGI"
    },
    {
      "provider": "devpass",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "DevPass"
    },
    {
      "provider": "atlascloud",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Atlas Cloud"
    },
    {
      "provider": "vercel",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Vercel AI Gateway"
    },
    {
      "provider": "llmman",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "llmman"
    },
    {
      "provider": "xkiro",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "xKiro"
    },
    {
      "provider": "museai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Muse (muse.ai)"
    },
    {
      "provider": "lithosai",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "LithosAI"
    },
    {
      "provider": "workbuddy",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "WorkBuddy"
    },
    {
      "provider": "tavily",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Tavily"
    },
    {
      "provider": "linkup",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Linkup"
    },
    {
      "provider": "tinyapi",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "TinyApi"
    },
    {
      "provider": "exa",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Exa"
    },
    {
      "provider": "cosmic",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Cosmic AI"
    },
    {
      "provider": "aerostack",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Aerostack"
    },
    {
      "provider": "sailresearch",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Sail Research"
    },
    {
      "provider": "sofya",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Sofya"
    },
    {
      "provider": "fal",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "fal"
    },
    {
      "provider": "flora",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "FLORA"
    },
    {
      "provider": "parallel",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Parallel"
    },
    {
      "provider": "claudeplatform",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Claude Platform"
    },
    {
      "provider": "munito",
      "enabled": false,
      "defaultEnabled": false,
      "displayName": "Munito"
    }
  ],
  "history": {
    "source": "muse:configured",
    "days": [
      {
        "day": "2026-10-10",
        "tokens": 420000
      },
      {
        "day": "2026-10-11",
        "tokens": 610000
      },
      {
        "day": "2026-10-12",
        "tokens": null
      },
      {
        "day": "2026-10-13",
        "tokens": 980000
      },
      {
        "day": "2026-10-14",
        "tokens": 720000
      },
      {
        "day": "2026-10-15",
        "tokens": 1180000
      },
      {
        "day": "2026-10-16",
        "tokens": 830000
      }
    ],
    "totalTokens": 4740000,
    "models": [
      {
        "name": "Model A",
        "input": 1610000,
        "output": 890000
      },
      {
        "name": "Model B",
        "input": 1240000,
        "output": 650000
      },
      {
        "name": "Model C",
        "input": 210000,
        "output": 140000
      }
    ]
  }
};
