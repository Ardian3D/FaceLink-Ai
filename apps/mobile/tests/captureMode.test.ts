import { describe, expect, it } from "vitest";
import { resolveCaptureMode } from "@/features/enrollment/captureMode";

describe("capture mode isolation", () => {
  it("forces SYNTHETIC_FLOW outside staging, regardless of env", () => {
    expect(resolveCaptureMode(false, "mannequin_preview_only")).toBe("SYNTHETIC_FLOW");
    expect(resolveCaptureMode(false, "MANNEQUIN_PREVIEW")).toBe("SYNTHETIC_FLOW");
  });

  it("defaults staging to SYNTHETIC_FLOW", () => {
    expect(resolveCaptureMode(true, undefined)).toBe("SYNTHETIC_FLOW");
    expect(resolveCaptureMode(true, "")).toBe("SYNTHETIC_FLOW");
    expect(resolveCaptureMode(true, "live")).toBe("SYNTHETIC_FLOW");
  });

  it("allows mannequin mode only through the explicit staging switch", () => {
    expect(resolveCaptureMode(true, "mannequin_preview_only")).toBe("MANNEQUIN_PREVIEW_ONLY");
    expect(resolveCaptureMode(true, "mannequin_preview")).toBe("MANNEQUIN_PREVIEW_ONLY");
  });
});
