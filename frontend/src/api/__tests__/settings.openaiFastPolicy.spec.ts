import { describe, expect, it } from "vitest";

import {
  buildOpenAIFastPolicySettingsPayload,
  type OpenAIFastPolicySettings,
} from "@/api/admin/settings";

describe("admin settings OpenAI fast policy payload helpers", () => {
  it("trims model whitelist and keeps fallback fields only when whitelist exists", () => {
    const form: OpenAIFastPolicySettings = {
      rules: [
        {
          service_tier: "priority",
          action: "block",
          scope: "oauth",
          error_message: "priority blocked",
          model_whitelist: [" gpt-5 ", "", "codex-*"],
          fallback_action: "block",
          fallback_error_message: "fallback blocked",
        },
        {
          service_tier: "flex",
          action: "filter",
          scope: "all",
          error_message: "ignored",
          model_whitelist: [" "],
          fallback_action: "block",
          fallback_error_message: "ignored fallback",
        },
      ],
    };

    expect(buildOpenAIFastPolicySettingsPayload(form)).toEqual({
      rules: [
        {
          service_tier: "priority",
          action: "block",
          scope: "oauth",
          error_message: "priority blocked",
          model_whitelist: ["gpt-5", "codex-*"],
          fallback_action: "block",
          fallback_error_message: "fallback blocked",
        },
        {
          service_tier: "flex",
          action: "filter",
          scope: "all",
          error_message: undefined,
          model_whitelist: undefined,
          fallback_action: undefined,
          fallback_error_message: undefined,
        },
      ],
    });
  });
});
