# CodexBar catalog classification review

Reviewer: Quotary implementer (primary review)

Date: 2026-10-20

CodexBar version: 0.77.0

Reference: `docs/reference/codexbar-0.77.0-providers.json`

Rubric: classify each provider as subscription, credits, metered API, mixed, non-usage, or unknown. Unknown is the conservative default and is neither bulk-trackable nor auto-polled. Unknown ActiveProbe means no provider-specific probe evidence was reviewed. A passive usage read is `ActiveProbeUnsupported`; an active/inference probe is `ActiveProbeSupported` with a reason and is outside the prototype. History is supported only for reviewed local readers.

Evidence method: pinned CodexBar 0.77.0 provider list plus product §3 priority table and broader-coverage list. Providers absent from that evidence remain unknown; no live credentials were used. Safe metering is derived from the reviewed kind: subscription/mixed sources use passive usage reads; credits/metered API remain individually trackable but are excluded from bulk tracking; unknown kinds are neither bulk-tracked nor auto-polled. The presentation hint is the manifest display name.

| Provider | Display name | Kind | Variants | History | Active probe | Evidence / probe reason |
| --- | --- | --- | --- | --- | --- | --- |
| `abacus` | Abacus AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aerostack` | Aerostack | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aiand` | ai& | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aixy` | Aixy | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `alibaba` | Alibaba | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `alibabatokenplan` | Alibaba Token Plan | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `amp` | Amp | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `antigravity` | Antigravity | subscription | configured | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `atlascloud` | Atlas Cloud | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `augment` | Augment | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `azureopenai` | Azure OpenAI | metered_api | configured | unknown | supported | Azure OpenAI reads may issue a billable chat-completion probe; outside the prototype until reviewed. |
| `bedrock` | AWS Bedrock | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `bifrost` | Bifrost | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `chutes` | Chutes | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `claude` | Claude | subscription | configured | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `claudeplatform` | Claude Platform | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `clawrouter` | ClawRouter | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `clinepass` | ClinePass | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `codebuff` | Codebuff | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `coderabbit` | CodeRabbit | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `codex` | Codex | subscription | configured | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `commandcode` | Command Code | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `copilot` | Copilot | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `cosmic` | Cosmic AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `cursor` | Cursor | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `deepgram` | Deepgram | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `deepinfra` | DeepInfra | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `deepseek` | DeepSeek | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `devin` | Devin | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `devpass` | DevPass | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `doubao` | Doubao | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `elevenlabs` | ElevenLabs | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `exa` | Exa | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `factory` | Droid | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `fal` | fal | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `fireworks` | Fireworks | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `flora` | FLORA | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `futureagi` | Future AGI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `gemini` | Gemini | subscription | Configured,Web,API | unknown | unsupported | Passive usage read for Gemini app and CLI source variants; no inference probe. |
| `gitkraken` | GitKraken AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `grok` | Grok | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `groq` | Groq | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `helmcode` | Helmcode | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `huggingface` | Hugging Face | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `hyper` | Charm Hyper | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `ibmbob` | IBM Bob | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `jetbrains` | JetBrains AI | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kilo` | Kilo | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kimi` | Kimi Code | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kiro` | Kiro | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `langdock` | Langdock | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `linkup` | Linkup | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `litellm` | LiteLLM | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `lithosai` | LithosAI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `llmman` | llmman | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `llmproxy` | LLM Proxy | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `longcat` | LongCat | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `manus` | Manus | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `mimo` | Xiaomi MiMo | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `minimax` | MiniMax | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `mistral` | Mistral | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `moonshot` | Moonshot / Kimi Open Platform | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `munito` | Munito | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `muse` | Muse Code | subscription | configured | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `museai` | Muse (muse.ai) | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `neuralwatt` | Neuralwatt | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `notion` | Notion AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `nous` | Nous Portal | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `ollama` | Ollama | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `openai` | OpenAI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `opencode` | OpenCode | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `opencodego` | OpenCode Go | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `openrouter` | OpenRouter | credits | configured | unknown | unsupported | Credit balance service; passive credit read, no inference probe. |
| `parallel` | Parallel | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `perplexity` | Perplexity | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `pi` | Pi | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `poe` | Poe | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `qoder` | Qoder | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `qwencloud` | Qwen Cloud | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `raycast` | Raycast | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `replicate` | Replicate | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sailresearch` | Sail Research | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sakana` | Sakana AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sofya` | Sofya | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `stepfun` | StepFun | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sub2api` | sub2api | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `synthetic` | Synthetic | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `t3chat` | T3 Chat | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `tavily` | Tavily | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `tinyapi` | TinyApi | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `typesafe` | TypeSafe | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `v0` | v0 | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `venice` | Venice | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `vercel` | Vercel AI Gateway | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `vertexai` | Vertex AI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `warp` | Warp | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `wayfinder` | Wayfinder | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `windsurf` | Windsurf | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `workbuddy` | WorkBuddy | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xai` | xAI | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xapi` | X API | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xkiro` | xKiro | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `zai` | z.ai / GLM | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `zed` | Zed | subscription | configured | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `zenmux` | ZenMux | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `zoommate` | ZoomMate | unknown | configured | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
