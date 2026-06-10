package service

import "testing"

func TestDeepSeekAccountOpenAICompatibleCapabilities(t *testing.T) {
	account := &Account{
		Platform: PlatformDeepSeek,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	if !account.IsDeepSeek() {
		t.Fatal("expected DeepSeek account")
	}
	if !account.IsOpenAIChatCompletionsCompatible() {
		t.Fatal("expected OpenAI-compatible Chat Completions account")
	}
	if !account.IsOpenAIApiKey() {
		t.Fatal("expected DeepSeek API key to be exposed through OpenAI-compatible API key helper")
	}
	if got := account.GetOpenAIBaseURL(); got != DefaultDeepSeekBaseURL {
		t.Fatalf("base URL = %q, want %q", got, DefaultDeepSeekBaseURL)
	}
	if got := account.GetOpenAIApiKey(); got != "sk-test" {
		t.Fatalf("api key = %q, want sk-test", got)
	}
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
		t.Fatal("expected DeepSeek to support Chat Completions")
	}
	if account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings) {
		t.Fatal("expected DeepSeek to reject Embeddings")
	}
	if account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic) {
		t.Fatal("expected DeepSeek to reject Images")
	}
}
