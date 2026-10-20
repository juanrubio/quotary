# CodexBar catalog classification review

Reviewer: Quotary implementer (primary review)

Date: 2026-10-20

CodexBar version: 0.77.0

Reference: `docs/reference/codexbar-0.77.0-providers.json`

Rubric: classify each provider as subscription, credits, metered API, mixed, non-usage, or unknown. Unknown is the conservative default and is neither bulk-trackable nor auto-polled. Unknown ActiveProbe means no provider-specific probe evidence was reviewed. A passive usage read is `ActiveProbeUnsupported`; an active/inference probe is `ActiveProbeSupported` with a reason and is outside the prototype. History is supported only for reviewed local readers.

Evidence method: pinned CodexBar 0.77.0 provider list plus product §3 priority table and broader-coverage list. Providers absent from that evidence remain unknown; no live credentials were used. Safe metering is derived from the reviewed kind: subscription/mixed sources use passive usage reads; credits/metered API remain individually trackable but are excluded from bulk tracking; unknown kinds are neither bulk-tracked nor auto-polled. The presentation hint is the manifest display name.

| Provider | Display name | Kind | Variants | History | Active probe | Evidence / probe reason |
| --- | --- | --- | --- | --- | --- | --- |
| `abacus` | Abacus AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aerostack` | Aerostack | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aiand` | ai& | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `aixy` | Aixy | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `alibaba` | Alibaba | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `alibabatokenplan` | Alibaba Token Plan | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `amp` | Amp | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `antigravity` | Antigravity | subscription | bare | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `atlascloud` | Atlas Cloud | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `augment` | Augment | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `azureopenai` | Azure OpenAI | metered_api | bare | unknown | supported | Azure OpenAI reads may issue a billable chat-completion probe; outside the prototype until reviewed. |
| `bedrock` | AWS Bedrock | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `bifrost` | Bifrost | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `chutes` | Chutes | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `claude` | Claude | subscription | bare | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `claudeplatform` | Claude Platform | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `clawrouter` | ClawRouter | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `clinepass` | ClinePass | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `codebuff` | Codebuff | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `coderabbit` | CodeRabbit | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `codex` | Codex | subscription | bare | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `commandcode` | Command Code | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `copilot` | Copilot | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `cosmic` | Cosmic AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `cursor` | Cursor | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `deepgram` | Deepgram | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `deepinfra` | DeepInfra | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `deepseek` | DeepSeek | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `devin` | Devin | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `devpass` | DevPass | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `doubao` | Doubao | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `elevenlabs` | ElevenLabs | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `exa` | Exa | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `factory` | Droid | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `fal` | fal | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `fireworks` | Fireworks | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `flora` | FLORA | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `futureagi` | Future AGI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `gemini` | Gemini | subscription | bare,web,api | unknown | unsupported | Passive usage read for Gemini app and CLI source variants; no inference probe. |
| `gitkraken` | GitKraken AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `grok` | Grok | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `groq` | Groq | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `helmcode` | Helmcode | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `huggingface` | Hugging Face | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `hyper` | Charm Hyper | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `ibmbob` | IBM Bob | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `jetbrains` | JetBrains AI | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kilo` | Kilo | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kimi` | Kimi Code | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `kiro` | Kiro | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `langdock` | Langdock | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `linkup` | Linkup | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `litellm` | LiteLLM | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `lithosai` | LithosAI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `llmman` | llmman | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `llmproxy` | LLM Proxy | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `longcat` | LongCat | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `manus` | Manus | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `mimo` | Xiaomi MiMo | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `minimax` | MiniMax | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `mistral` | Mistral | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `moonshot` | Moonshot / Kimi Open Platform | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `munito` | Munito | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `muse` | Muse Code | subscription | bare | supported | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `museai` | Muse (muse.ai) | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `neuralwatt` | Neuralwatt | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `notion` | Notion AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `nous` | Nous Portal | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `ollama` | Ollama | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `openai` | OpenAI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `opencode` | OpenCode | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `opencodego` | OpenCode Go | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `openrouter` | OpenRouter | credits | bare | unknown | unsupported | Credit balance service; passive credit read, no inference probe. |
| `parallel` | Parallel | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `perplexity` | Perplexity | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `pi` | Pi | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `poe` | Poe | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `qoder` | Qoder | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `qwencloud` | Qwen Cloud | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `raycast` | Raycast | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `replicate` | Replicate | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sailresearch` | Sail Research | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sakana` | Sakana AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sofya` | Sofya | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `stepfun` | StepFun | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `sub2api` | sub2api | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `synthetic` | Synthetic | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `t3chat` | T3 Chat | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `tavily` | Tavily | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `tinyapi` | TinyApi | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `typesafe` | TypeSafe | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `v0` | v0 | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `venice` | Venice | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `vercel` | Vercel AI Gateway | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `vertexai` | Vertex AI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `warp` | Warp | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `wayfinder` | Wayfinder | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `windsurf` | Windsurf | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `workbuddy` | WorkBuddy | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xai` | xAI | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xapi` | X API | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `xkiro` | xKiro | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `zai` | z.ai / GLM | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `zed` | Zed | subscription | bare | unknown | unsupported | Product §3 broader-coverage subscription; passive usage read only. |
| `zenmux` | ZenMux | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
| `zoommate` | ZoomMate | unknown | bare | unknown | unknown | No provider-specific usage/probe evidence reviewed; conservative unknown |
