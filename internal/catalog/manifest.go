package catalog

import "github.com/juanrubio/quotary/internal/domain"

// Entry is the reviewed capability annotation for one CodexBar provider ID.
type Entry struct {
	ProviderID  string
	DisplayName string
	Kind        domain.ServiceKind
	Variants    []domain.Variant
	History     domain.CapabilityState
	ActiveProbe domain.CapabilityState
	ProbeReason string
}

// Manifest returns a copy of the pinned, reviewed provider manifest.
func Manifest() map[string]Entry {
	result := make(map[string]Entry, len(manifest))
	for key, entry := range manifest {
		entry.Variants = append([]domain.Variant(nil), entry.Variants...)
		result[key] = entry
	}
	return result
}

var manifest = map[string]Entry{
	"codex":            {ProviderID: "codex", DisplayName: "Codex", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"openai":           {ProviderID: "openai", DisplayName: "OpenAI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"azureopenai":      {ProviderID: "azureopenai", DisplayName: "Azure OpenAI", Kind: domain.KindMeteredAPI, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilitySupported, ProbeReason: "Azure OpenAI reads may issue a billable chat-completion probe; outside the prototype until reviewed."},
	"claude":           {ProviderID: "claude", DisplayName: "Claude", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"clinepass":        {ProviderID: "clinepass", DisplayName: "ClinePass", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"cursor":           {ProviderID: "cursor", DisplayName: "Cursor", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"opencode":         {ProviderID: "opencode", DisplayName: "OpenCode", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"opencodego":       {ProviderID: "opencodego", DisplayName: "OpenCode Go", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"alibaba":          {ProviderID: "alibaba", DisplayName: "Alibaba", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"alibabatokenplan": {ProviderID: "alibabatokenplan", DisplayName: "Alibaba Token Plan", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"qwencloud":        {ProviderID: "qwencloud", DisplayName: "Qwen Cloud", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"factory":          {ProviderID: "factory", DisplayName: "Droid", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"fireworks":        {ProviderID: "fireworks", DisplayName: "Fireworks", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"gemini":           {ProviderID: "gemini", DisplayName: "Gemini", Kind: domain.KindSubscription, Variants: []domain.Variant{domain.VariantConfigured, domain.VariantWeb, domain.VariantAPI}, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Passive usage read for Gemini app and CLI source variants; no inference probe."},
	"antigravity":      {ProviderID: "antigravity", DisplayName: "Antigravity", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"copilot":          {ProviderID: "copilot", DisplayName: "Copilot", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"devin":            {ProviderID: "devin", DisplayName: "Devin", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"zai":              {ProviderID: "zai", DisplayName: "z.ai / GLM", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"minimax":          {ProviderID: "minimax", DisplayName: "MiniMax", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"manus":            {ProviderID: "manus", DisplayName: "Manus", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"kimi":             {ProviderID: "kimi", DisplayName: "Kimi Code", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"kilo":             {ProviderID: "kilo", DisplayName: "Kilo", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"kiro":             {ProviderID: "kiro", DisplayName: "Kiro", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"vertexai":         {ProviderID: "vertexai", DisplayName: "Vertex AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"augment":          {ProviderID: "augment", DisplayName: "Augment", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"jetbrains":        {ProviderID: "jetbrains", DisplayName: "JetBrains AI", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"moonshot":         {ProviderID: "moonshot", DisplayName: "Moonshot / Kimi Open Platform", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"amp":              {ProviderID: "amp", DisplayName: "Amp", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"t3chat":           {ProviderID: "t3chat", DisplayName: "T3 Chat", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"langdock":         {ProviderID: "langdock", DisplayName: "Langdock", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"ollama":           {ProviderID: "ollama", DisplayName: "Ollama", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"synthetic":        {ProviderID: "synthetic", DisplayName: "Synthetic", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"openrouter":       {ProviderID: "openrouter", DisplayName: "OpenRouter", Kind: domain.KindCredits, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Credit balance service; passive credit read, no inference probe."},
	"elevenlabs":       {ProviderID: "elevenlabs", DisplayName: "ElevenLabs", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"warp":             {ProviderID: "warp", DisplayName: "Warp", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"windsurf":         {ProviderID: "windsurf", DisplayName: "Windsurf", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"zed":              {ProviderID: "zed", DisplayName: "Zed", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"perplexity":       {ProviderID: "perplexity", DisplayName: "Perplexity", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"mimo":             {ProviderID: "mimo", DisplayName: "Xiaomi MiMo", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"doubao":           {ProviderID: "doubao", DisplayName: "Doubao", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"sakana":           {ProviderID: "sakana", DisplayName: "Sakana AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"abacus":           {ProviderID: "abacus", DisplayName: "Abacus AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"mistral":          {ProviderID: "mistral", DisplayName: "Mistral", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"deepseek":         {ProviderID: "deepseek", DisplayName: "DeepSeek", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"deepinfra":        {ProviderID: "deepinfra", DisplayName: "DeepInfra", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"codebuff":         {ProviderID: "codebuff", DisplayName: "Codebuff", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"venice":           {ProviderID: "venice", DisplayName: "Venice", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"commandcode":      {ProviderID: "commandcode", DisplayName: "Command Code", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"qoder":            {ProviderID: "qoder", DisplayName: "Qoder", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"stepfun":          {ProviderID: "stepfun", DisplayName: "StepFun", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"bedrock":          {ProviderID: "bedrock", DisplayName: "AWS Bedrock", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"grok":             {ProviderID: "grok", DisplayName: "Grok", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"groq":             {ProviderID: "groq", DisplayName: "Groq", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"llmproxy":         {ProviderID: "llmproxy", DisplayName: "LLM Proxy", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"litellm":          {ProviderID: "litellm", DisplayName: "LiteLLM", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"bifrost":          {ProviderID: "bifrost", DisplayName: "Bifrost", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"aixy":             {ProviderID: "aixy", DisplayName: "Aixy", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"deepgram":         {ProviderID: "deepgram", DisplayName: "Deepgram", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"poe":              {ProviderID: "poe", DisplayName: "Poe", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"chutes":           {ProviderID: "chutes", DisplayName: "Chutes", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"neuralwatt":       {ProviderID: "neuralwatt", DisplayName: "Neuralwatt", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"helmcode":         {ProviderID: "helmcode", DisplayName: "Helmcode", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"clawrouter":       {ProviderID: "clawrouter", DisplayName: "ClawRouter", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"longcat":          {ProviderID: "longcat", DisplayName: "LongCat", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"sub2api":          {ProviderID: "sub2api", DisplayName: "sub2api", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"wayfinder":        {ProviderID: "wayfinder", DisplayName: "Wayfinder", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"zenmux":           {ProviderID: "zenmux", DisplayName: "ZenMux", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"aiand":            {ProviderID: "aiand", DisplayName: "ai&", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"zoommate":         {ProviderID: "zoommate", DisplayName: "ZoomMate", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"xai":              {ProviderID: "xai", DisplayName: "xAI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"xapi":             {ProviderID: "xapi", DisplayName: "X API", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"notion":           {ProviderID: "notion", DisplayName: "Notion AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"ibmbob":           {ProviderID: "ibmbob", DisplayName: "IBM Bob", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"nous":             {ProviderID: "nous", DisplayName: "Nous Portal", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"muse":             {ProviderID: "muse", DisplayName: "Muse Code", Kind: domain.KindSubscription, Variants: nil, History: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported, ProbeReason: "Product \u00a73 broader-coverage subscription; passive usage read only."},
	"coderabbit":       {ProviderID: "coderabbit", DisplayName: "CodeRabbit", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"replicate":        {ProviderID: "replicate", DisplayName: "Replicate", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"huggingface":      {ProviderID: "huggingface", DisplayName: "Hugging Face", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"raycast":          {ProviderID: "raycast", DisplayName: "Raycast", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"pi":               {ProviderID: "pi", DisplayName: "Pi", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"v0":               {ProviderID: "v0", DisplayName: "v0", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"typesafe":         {ProviderID: "typesafe", DisplayName: "TypeSafe", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"hyper":            {ProviderID: "hyper", DisplayName: "Charm Hyper", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"gitkraken":        {ProviderID: "gitkraken", DisplayName: "GitKraken AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"futureagi":        {ProviderID: "futureagi", DisplayName: "Future AGI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"devpass":          {ProviderID: "devpass", DisplayName: "DevPass", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"atlascloud":       {ProviderID: "atlascloud", DisplayName: "Atlas Cloud", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"vercel":           {ProviderID: "vercel", DisplayName: "Vercel AI Gateway", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"llmman":           {ProviderID: "llmman", DisplayName: "llmman", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"xkiro":            {ProviderID: "xkiro", DisplayName: "xKiro", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"museai":           {ProviderID: "museai", DisplayName: "Muse (muse.ai)", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"lithosai":         {ProviderID: "lithosai", DisplayName: "LithosAI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"workbuddy":        {ProviderID: "workbuddy", DisplayName: "WorkBuddy", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"tavily":           {ProviderID: "tavily", DisplayName: "Tavily", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"linkup":           {ProviderID: "linkup", DisplayName: "Linkup", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"tinyapi":          {ProviderID: "tinyapi", DisplayName: "TinyApi", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"exa":              {ProviderID: "exa", DisplayName: "Exa", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"cosmic":           {ProviderID: "cosmic", DisplayName: "Cosmic AI", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"aerostack":        {ProviderID: "aerostack", DisplayName: "Aerostack", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"sailresearch":     {ProviderID: "sailresearch", DisplayName: "Sail Research", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"sofya":            {ProviderID: "sofya", DisplayName: "Sofya", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"fal":              {ProviderID: "fal", DisplayName: "fal", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"flora":            {ProviderID: "flora", DisplayName: "FLORA", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"parallel":         {ProviderID: "parallel", DisplayName: "Parallel", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"claudeplatform":   {ProviderID: "claudeplatform", DisplayName: "Claude Platform", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
	"munito":           {ProviderID: "munito", DisplayName: "Munito", Kind: domain.KindUnknown, Variants: nil, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown, ProbeReason: ""},
}

// Classification rubric: subscription, credits, metered API, non-usage, or
// unknown. Unknown is conservative and is neither bulk-trackable nor
// automatically polled. Passive usage reads are ActiveProbeUnsupported; active
// probes are Supported with ProbeReason and stay outside the prototype.
