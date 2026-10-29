import { afterEach, describe, expect, it, vi } from "vitest";
import { createRequire } from "node:module";
import crypto from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { decodeBase64 } from "@/features/enrollment/fixture";

import { createHash } from "node:crypto";
import ts from "typescript";
const require = createRequire(import.meta.url);

function findPrepare(): string {
  let directory = path.resolve(import.meta.dirname);
  for (let i = 0; i < 8; i++) {
    const candidate = path.join(directory, "research", "fl-enr02a-host", "prepare-enr02a.cjs");
    if (fs.existsSync(candidate)) return candidate;
    directory = path.dirname(directory);
  }
  throw new Error("prepare-enr02a.cjs tidak ditemukan");
}

const prepare = require(findPrepare()) as {
  publicApi(value: string): string;
  stageManifest(value: string, allowCamera?: boolean): string;
  stageMainActivity(value: string): string;
  IDENTITY: { applicationId: string; scheme: string };
  ENR06_FIXTURE: { path: string; sha256: string; minBytes: number; maxBytes: number };
  offlineFixtureEnabled(env: Record<string, string | undefined>, mode: string, adminReview: boolean): boolean;
  readPinnedFixture(): Buffer;
  stageOfflineFixture(source: string): { source: string; provenance: Record<string, unknown> };
  verifyFixtureBytes(file: string, sha256: string, minBytes: number, maxBytes: number): Buffer;
};

const pinnedFixturePath = path.join(
  path.resolve(path.dirname(findPrepare()), "../.."),
  prepare.ENR06_FIXTURE.path
);

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
  vi.restoreAllMocks();
});

describe("FL-ENR02A staging isolation transforms", () => {
  it("uses a package and scheme distinct from the main app", () => {
    expect(prepare.IDENTITY.applicationId).toBe("ai.facelink.enr02host");
    expect(prepare.IDENTITY.applicationId).not.toBe("ai.facelink.mobile");
    expect(prepare.IDENTITY.scheme).toBe("facelinkenr02");
  });

  it.each([
    "http://127.0.0.1:8091",
    "https://user:password@127.0.0.1:8091",
    "https://127.0.0.1:8091/?token=secret",
    "https://127.0.0.1:8091/#secret",
    "https://192.168.1.5:8091",
  ])("rejects unsafe staging API input without exposing it: %s", (value) => {
    expect(() => prepare.publicApi(value)).toThrow();
  });

  it("accepts only HTTPS loopback staging addresses", () => {
    expect(prepare.publicApi("https://127.0.0.1:8091/")).toBe("https://127.0.0.1:8091");
    expect(prepare.publicApi("https://localhost:8091")).toBe("https://localhost:8091");
  });

  it("keeps INTERNET and camera, blocks media, and binds the actual host classes", () => {
    const source = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">\n  <uses-permission android:name="android.permission.INTERNET"/>\n  <uses-permission android:name="android.permission.CAMERA"/>\n  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES"/>\n  <application android:name=".MainApplication"><activity android:name=".MainActivity"><intent-filter><data android:scheme="facelinkai"/><data android:scheme="exp+facelink-ai"/></intent-filter></activity></application>\n</manifest>`;
    const transformed = prepare.stageManifest(source, true);
    expect(transformed).toContain("ai.facelink.mobile.MainApplication");
    expect(transformed).toContain("ai.facelink.mobile.MainActivity");
    expect(transformed).toContain('android:scheme="facelinkenr02"');
    expect(transformed).toContain("android.permission.INTERNET");
    expect(transformed).toContain('<uses-permission android:name="android.permission.CAMERA"/>');
    expect(transformed).not.toContain('android:name="android.permission.CAMERA" tools:node="remove"');
    expect(transformed).toContain('android:name="android.permission.READ_MEDIA_IMAGES" tools:node="remove"');
  });

  it("blocks camera in synthetic staging and sets secure activity only for live mode", () => {
    const source = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">\n  <uses-permission android:name="android.permission.INTERNET"/>\n  <application android:name=".MainApplication"><activity android:name=".MainActivity"><intent-filter><data android:scheme="facelinkai"/><data android:scheme="exp+facelink-ai"/></intent-filter></activity></application>\n</manifest>`;
    const synthetic = prepare.stageManifest(source);
    expect(synthetic).not.toContain('<uses-permission android:name="android.permission.CAMERA"/>');
    expect(synthetic).toContain('android:name="android.permission.CAMERA" tools:node="remove"');
    expect(prepare.stageMainActivity("    super.onCreate(null)")).toContain("FLAG_SECURE");
  });

  it("defaults the dashboard staging flag to false", async () => {
    vi.resetModules();
    vi.stubEnv("EXPO_PUBLIC_FACELINK_ENR_STAGING", undefined);
    const off = await import("@/features/enrollment/staging");
    expect(off.enrollmentStagingAktif).toBe(false);
    vi.resetModules();
    vi.stubEnv("EXPO_PUBLIC_FACELINK_ENR_STAGING", "1");
    const on = await import("@/features/enrollment/staging");
    expect(on.enrollmentStagingAktif).toBe(true);
    vi.unstubAllEnvs();
  });
});

function evaluateModule<T>(source: string, dependencies: Record<string, unknown> = {}): T {
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports = {};
  new Function("exports", "require", compiled)(exports, (name: string) => {
    if (!(name in dependencies)) throw new Error("Unexpected generated-module dependency");
    return dependencies[name];
  });
  return exports as T;
}

describe("FL-ENR06 pinned offline integration fixture", () => {
  it("requires explicit private opt-in, synthetic mode, and Admin review", () => {
    expect(prepare.offlineFixtureEnabled({}, "synthetic_flow", true)).toBe(false);
    expect(prepare.offlineFixtureEnabled({ FACELINK_ENR06_FIXTURE: "0" }, "synthetic_flow", true)).toBe(false);
    expect(prepare.offlineFixtureEnabled({ FACELINK_ENR06_FIXTURE: "1" }, "synthetic_flow", true)).toBe(true);
    expect(() => prepare.offlineFixtureEnabled({ FACELINK_ENR06_FIXTURE: "true" }, "synthetic_flow", true)).toThrow();
    expect(() => prepare.offlineFixtureEnabled({ FACELINK_ENR06_FIXTURE: "1" }, "synthetic_flow", false)).toThrow();
    expect(() => prepare.offlineFixtureEnabled({ FACELINK_ENR06_FIXTURE: "1" }, "mannequin_preview_only", true)).toThrow();
  });

  it.skipIf(!fs.existsSync(pinnedFixturePath))("executes the actual generated module and delivers pinned bytes through guided capture once", () => {
    const root = path.resolve(path.dirname(findPrepare()), "../..");
    const fixtureSource = fs.readFileSync(path.join(root, "apps/mobile/src/features/enrollment/fixture.ts"), "utf8");
    const generated = prepare.stageOfflineFixture(fixtureSource);
    const fixture = evaluateModule<typeof import("@/features/enrollment/fixture")>(generated.source);
    const first = fixture.syntheticJPEG();
    const second = fixture.syntheticJPEG();
    expect(first).not.toBe(second);
    expect(createHash("sha256").update(first).digest("hex")).toBe(prepare.ENR06_FIXTURE.sha256);
    expect(Buffer.from(first).equals(prepare.readPinnedFixture())).toBe(true);
    expect(first.length).toBeGreaterThanOrEqual(prepare.ENR06_FIXTURE.minBytes);
    expect(first.length).toBeLessThanOrEqual(prepare.ENR06_FIXTURE.maxBytes);
    expect(Array.from(first.subarray(0, 2))).toEqual([255, 216]);
    expect(Array.from(first.subarray(-2))).toEqual([255, 217]);
    expect(Array.from(fixture.decodeBase64("AQID"))).toEqual([1, 2, 3]);
    expect(() => fixture.decodeBase64("!bad")).toThrow();
    const captureSource = fs.readFileSync(path.join(root, "apps/mobile/src/features/enrollment/syntheticCapture.ts"), "utf8");
    const { SyntheticCapture } = evaluateModule<typeof import("@/features/enrollment/syntheticCapture")>(
      captureSource, { "./fixture": fixture },
    );
    const capture = new SyntheticCapture();
    capture.start(0);
    expect(capture.consume(1_000)).toBeNull();
    const captured = capture.consume(2_400);
    expect(captured).not.toBeNull();
    expect(createHash("sha256").update(captured!).digest("hex")).toBe(prepare.ENR06_FIXTURE.sha256);
    expect(capture.consume(3_000)).toBeNull();
    const cancelled = new SyntheticCapture();
    cancelled.start(0);
    cancelled.cancel();
    expect(cancelled.consume(3_000)).toBeNull();
    expect(generated.provenance).toMatchObject({
      optIn: "FACELINK_ENR06_FIXTURE=1", camera: false, downloads: false,
      sha256: prepare.ENR06_FIXTURE.sha256, captureMode: "synthetic_flow", adminReview: true,
    });
  });

  it("accepts only a regular file whose bytes match the expected lowercase hash", () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), "enr06-fixture-"));
    try {
      const file = path.join(directory, "fixture.jpg");
      const bytes = Buffer.alloc(1_200, 7);
      fs.writeFileSync(file, bytes);
      const sha256 = createHash("sha256").update(bytes).digest("hex");
      expect(prepare.verifyFixtureBytes(file, sha256, 1_001, 1_048_575).equals(bytes)).toBe(true);
      expect(() => prepare.verifyFixtureBytes(file, sha256.toUpperCase(), 1_001, 1_048_575)).toThrow(/lowercase/);
      expect(() => prepare.verifyFixtureBytes(file, "0".repeat(64), 1_001, 1_048_575)).toThrow(/mismatch/);
    } finally {
      fs.rmSync(directory, { recursive: true, force: true });
    }
  });

  it("rejects missing, nonregular, and out-of-bound inputs", () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), "enr06-fixture-"));
    try {
      const file = path.join(directory, "fixture.jpg");
      const bytes = Buffer.alloc(1_200, 7);
      fs.writeFileSync(file, bytes);
      const sha256 = createHash("sha256").update(bytes).digest("hex");
      expect(() => prepare.verifyFixtureBytes(path.join(directory, "missing.jpg"), sha256, 1_001, 1_048_575)).toThrow(/missing/);
      expect(() => prepare.verifyFixtureBytes(directory, sha256, 1_001, 1_048_575)).toThrow(/regular file/);
      expect(() => prepare.verifyFixtureBytes(file, sha256, 1_201, 1_048_575)).toThrow(/byte bounds/);
      expect(() => prepare.verifyFixtureBytes(file, sha256, 1_001, 1_199)).toThrow(/byte bounds/);
      expect(() => prepare.verifyFixtureBytes(file, sha256, 0, 1_048_575)).toThrow(/invalid/);
      expect(() => prepare.verifyFixtureBytes(file, sha256, 1_001, 1_048_577)).toThrow(/invalid/);
    } finally {
      fs.rmSync(directory, { recursive: true, force: true });
    }
  });

  it.skipIf(process.platform === "win32")("rejects a fixture reached through a symbolic link", () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), "enr06-fixture-"));
    try {
      const target = path.join(directory, "target.jpg");
      const bytes = Buffer.alloc(1_200, 7);
      fs.writeFileSync(target, bytes);
      const link = path.join(directory, "link.jpg");
      fs.symlinkSync(target, link);
      const sha256 = createHash("sha256").update(bytes).digest("hex");
      expect(() => prepare.verifyFixtureBytes(link, sha256, 1_001, 1_048_575)).toThrow(/symbolic links/);
    } finally {
      fs.rmSync(directory, { recursive: true, force: true });
    }
  });
});
