import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

// All imported auth/transport/router modules are the actual copied FaceLink modules.
// Only platform SecureStore and HTTP boundaries are fixtures; no real backend calls.
const secure = vi.hoisted(() => ({
  isAvailableAsync: vi.fn(async () => true),
  getItemAsync: vi.fn(async () => null),
  setItemAsync: vi.fn(), deleteItemAsync: vi.fn(),
}));
vi.mock('expo-secure-store', () => ({ ...secure, WHEN_UNLOCKED_THIS_DEVICE_ONLY: 1 }));
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.clearAllMocks(); });

describe('actual-host staging regression', () => {
  it('starts with an empty isolated SecureStore without writing a fabricated session', async () => {
    vi.resetModules();
    const store = await import('@/features/auth/tokenStore');
    expect(await store.muatSesiTersimpan()).toBeNull();
    expect(await store.muatSesiTersimpan()).toBeNull();
    expect(secure.getItemAsync).toHaveBeenCalledTimes(1);
    expect(secure.setItemAsync).not.toHaveBeenCalled();
    expect(secure.deleteItemAsync).not.toHaveBeenCalled();
  });

  it('passes normalized email through real OTP flow and real HTTPS transport without starting Google or choosing a role', async () => {
    vi.resetModules();
    vi.stubEnv('EXPO_PUBLIC_FACELINK_API_BASE_URL', 'https://fixture.example.test');
    vi.stubGlobal('__DEV__', false);
    const fetchFixture = vi.fn(async () => new Response(JSON.stringify({ data: {
      challenge_id: '11111111-1111-4111-8111-111111111111', expires_in: 300, retry_after: 60,
    } }), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchFixture);
    const api = await import('@/features/auth/authApi');
    const { EmailOtpFlow } = await import('@/features/auth/emailOtpFlow');
    const { ruteAlihRole } = await import('@/features/auth/roleRoutes');
    const flow = new EmailOtpFlow(api.mintaKodeEmail, async () => ({ ok: true, data: undefined }), () => 1000);
    await flow.minta('  Fixture@Example.Test  ');
    expect(flow.keadaan()).toMatchObject({ email: 'fixture@example.test', retryAt: 61000, expiresAt: 301000 });
    expect(fetchFixture).toHaveBeenCalledTimes(1);
    const args = fetchFixture.mock.calls[0] as unknown as [string, RequestInit];
    expect(args[0]).toBe('https://fixture.example.test/api/v1/auth/email/request');
    expect(JSON.parse(String(args[1].body))).toEqual({ email: 'fixture@example.test' });
    expect(ruteAlihRole(null, 'admin')).toBeNull();
    expect(secure.setItemAsync).not.toHaveBeenCalled();
  });

  it('blocks insecure host configuration before real transport can send any email', async () => {
    vi.resetModules();
    vi.stubEnv('EXPO_PUBLIC_FACELINK_API_BASE_URL', 'http://fixture.example.test');
    vi.stubGlobal('__DEV__', true);
    const fetchFixture = vi.fn();
    vi.stubGlobal('fetch', fetchFixture);
    const { mintaKodeEmail } = await import('@/features/auth/authApi');
    expect(await mintaKodeEmail('fixture@example.test')).toMatchObject({ ok: false, error: { kind: 'config' } });
    expect(fetchFixture).not.toHaveBeenCalled();
  });

  it('does not enable Google when its deliberately omitted public ID is missing', async () => {
    vi.resetModules();
    vi.stubEnv('EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID', undefined);
    const { konfigurasiGoogle } = await import('@/features/auth/googleConfig');
    expect(konfigurasiGoogle.ok).toBe(false);
  });
});

const require = createRequire(import.meta.url);
const prepare = require('../../prepare-host.cjs') as {
  publicApi(value: string): string;
  stageManifest(value: string): string;
};
describe('staging isolation transforms', () => {
  it.each(['http://fixture.test', 'https://user:password@fixture.test', 'https://fixture.test/?token=secret', 'https://fixture.test/#secret'])(
    'rejects unsafe public API input without exposing it: %s', (value) => expect(() => prepare.publicApi(value)).toThrow(),
  );
  it('transforms an actual-shaped Android fixture while retaining INTERNET and namespace-bound activity', () => {
    const source = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">\n  <uses-permission android:name="android.permission.INTERNET"/>\n  <uses-permission android:name="android.permission.CAMERA"/>\n  <application android:name=".MainApplication"><activity android:name=".MainActivity"><intent-filter><data android:scheme="facelinkai"/><data android:scheme="exp+facelink-ai"/></intent-filter></activity></application>\n</manifest>`;
    const transformed = prepare.stageManifest(source);
    expect(transformed).toContain('android.permission.INTERNET');
    expect(transformed).toContain('android:name="ai.facelink.mobile.MainActivity"');
    expect(transformed).toContain('android:scheme="facelinkcam03"');
    expect(transformed).toContain('android.permission.CAMERA" tools:node="remove"');
    expect(transformed).not.toContain('android.permission.CAMERA"/>');
  });
});

describe('actual host Metro video resolution', () => {
  it.each([true, false])('uses private local clips only when both files exist: %s', (available) => {
    const project = path.resolve(import.meta.dirname, '..');
    const config: { resolver: { resolveRequest?: (context: unknown, name: string, platform: string) => unknown }; watchFolders?: string[] } = { resolver: {} };
    const module = { exports: {} };
    vm.runInNewContext(fs.readFileSync(path.join(project, 'metro.config.js'), 'utf8'), {
      __dirname: project, module,
      require: (name: string) => {
        if (name === 'node:path') return path;
        if (name === 'node:fs') return { existsSync: () => available };
        if (name === 'expo/metro-config') return { getDefaultConfig: () => config };
        throw new Error(`Unexpected Metro dependency: ${name}`);
      },
    });
    const resolve = vi.fn((_context: unknown, moduleName: string) => moduleName);
    const context = { resolveRequest: resolve };
    const resolver = config.resolver.resolveRequest!;
    expect(resolver(context, '../../assets/video/video-clips', 'android')).toBe(
      path.join(project, 'assets/video', available ? 'video-clips.local.js' : 'video-clips.none.js'),
    );
    expect(resolver(context, 'expo-video', 'android')).toBe('expo-video');
    expect(resolver(context, 'expo-secure-store', 'android')).toBe('expo-secure-store');
    expect(resolver({ ...context, originModulePath: path.join(project, 'src/components/VideoBackground.tsx') }, 'expo-video', 'android')).toBe(
      path.join(project, 'src/cam03/video-observer.ts'),
    );
    expect(resolver({ ...context, originModulePath: path.join(project, 'src/cam03/video-observer.ts') }, 'expo-video', 'android')).toBe('expo-video');
    expect(resolver({ ...context, originModulePath: path.join(project, 'app/index.tsx') }, 'expo-video', 'android')).toBe('expo-video');
    // No video payload is read: only configuration JS is executed against file-existence fixtures.
  });
});
