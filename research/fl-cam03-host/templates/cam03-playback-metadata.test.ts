import { describe, expect, it, vi } from 'vitest';
import type { VideoPlayer, VideoPlayerEvents } from 'expo-video';

// Only the SDK event boundary is synthetic here. Tests execute the real staging collector.
function playerEvents() {
  const handlers = new Map<string, Set<(payload: never) => void>>();
  const player = {
    status: 'idle', playing: false, currentTime: 0,
    addListener(name: string, handler: (payload: never) => void) {
      const set = handlers.get(name) ?? new Set();
      set.add(handler); handlers.set(name, set);
      return { remove: () => { set.delete(handler); } };
    },
  };
  return {
    player: player as unknown as VideoPlayer,
    emit<K extends 'statusChange' | 'playingChange' | 'timeUpdate'>(name: K, payload: Parameters<VideoPlayerEvents[K]>[0]) {
      handlers.get(name)?.forEach((handler) => handler(payload as never));
    },
    listenerCount() { return [...handlers.values()].reduce((total, set) => total + set.size, 0); },
  };
}

describe('actual SDK observer metadata collector', () => {
  it('requires both players to actually advance while playing and records their playing overlap', async () => {
    vi.resetModules();
    const collector = await import('@/cam03/playback-metadata');
    const first = playerEvents(); const second = playerEvents();
    const detachFirst = collector.attachPlaybackMetadata(first.player);
    const detachSecond = collector.attachPlaybackMetadata(second.player);
    first.emit('statusChange', { status: 'readyToPlay' });
    second.emit('statusChange', { status: 'readyToPlay' });
    first.emit('timeUpdate', { currentTime: 3 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    expect(collector.getPlaybackMetadata().bothPlayersPlayedAndProgressed).toBe(false);
    first.emit('playingChange', { isPlaying: true });
    first.emit('timeUpdate', { currentTime: 4 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    second.emit('playingChange', { isPlaying: true });
    expect(collector.getPlaybackMetadata().simultaneousPlayingObserved).toBe(true);
    expect(collector.getPlaybackMetadata().bothPlayersPlayedAndProgressed).toBe(false);
    second.emit('timeUpdate', { currentTime: 0.2 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    expect(collector.getPlaybackMetadata()).toMatchObject({ bothPlayersPlayedAndProgressed: true, visualAppearanceVerified: false });
    expect(collector.getPlaybackMetadata().players.every((row) => row.readyObserved)).toBe(true);
    detachFirst(); detachSecond(); detachFirst();
    expect(first.listenerCount() + second.listenerCount()).toBe(0);
    const snapshot = collector.getPlaybackMetadata();
    first.emit('statusChange', { status: 'error' });
    expect(collector.getPlaybackMetadata()).toEqual(snapshot);
    expect(snapshot.players.every((row) => !row.attached && !row.playing)).toBe(true);
  });

  it('does not carry a previous mount into the next pair of players', async () => {
    vi.resetModules();
    const collector = await import('@/cam03/playback-metadata');
    const oldPair = [playerEvents(), playerEvents()];
    const oldDetach = oldPair.map((fixture) => collector.attachPlaybackMetadata(fixture.player));
    oldPair.forEach((fixture) => {
      fixture.emit('statusChange', { status: 'readyToPlay' });
      fixture.emit('playingChange', { isPlaying: true });
      fixture.emit('timeUpdate', { currentTime: 2 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    });
    expect(collector.getPlaybackMetadata()).toMatchObject({ bothPlayersPlayedAndProgressed: true, simultaneousPlayingObserved: true });
    oldDetach.forEach((detach) => detach());
    expect(collector.getPlaybackMetadata().bothPlayersPlayedAndProgressed).toBe(true);

    const newPair = [playerEvents(), playerEvents()];
    const newDetach = newPair.map((fixture) => collector.attachPlaybackMetadata(fixture.player));
    const current = collector.getPlaybackMetadata();
    expect(current).toMatchObject({ bothPlayersPlayedAndProgressed: false, simultaneousPlayingObserved: false, droppedPlayers: 0 });
    expect(current.players).toHaveLength(2);
    expect(current.players.every((row) => row.attached && !row.played)).toBe(true);
    newDetach.forEach((detach) => detach());
  });

  it('does not union an older still-mounted screen with the newest pair', async () => {
    vi.resetModules();
    const collector = await import('@/cam03/playback-metadata');
    const oldPair = [playerEvents(), playerEvents()];
    const oldDetach = oldPair.map((fixture) => collector.attachPlaybackMetadata(fixture.player));
    oldPair.forEach((fixture) => {
      fixture.emit('playingChange', { isPlaying: true });
      fixture.emit('timeUpdate', { currentTime: 2 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    });
    expect(collector.getPlaybackMetadata()).toMatchObject({ bothPlayersPlayedAndProgressed: true, simultaneousPlayingObserved: true });

    const newPair = [playerEvents(), playerEvents()];
    const newDetach = newPair.map((fixture) => collector.attachPlaybackMetadata(fixture.player));
    const beforeNewPlays = collector.getPlaybackMetadata();
    expect(beforeNewPlays).toMatchObject({ bothPlayersPlayedAndProgressed: false, simultaneousPlayingObserved: false });
    newPair.forEach((fixture) => {
      fixture.emit('playingChange', { isPlaying: true });
      fixture.emit('timeUpdate', { currentTime: 1 } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    });
    expect(collector.getPlaybackMetadata()).toMatchObject({ bothPlayersPlayedAndProgressed: true, simultaneousPlayingObserved: true });
    [...oldDetach, ...newDetach].forEach((detach) => detach());
  });

  it('bounds retained players, sanitizes numeric positions, and never retains error/source payloads', async () => {
    vi.resetModules();
    const collector = await import('@/cam03/playback-metadata');
    const cleanups: (() => void)[] = [];
    for (let index = 0; index < 12; index++) {
      const fixture = playerEvents();
      cleanups.push(collector.attachPlaybackMetadata(fixture.player));
      fixture.emit('statusChange', { status: 'error', error: { message: 'PRIVATE_SOURCE_URI_SENTINEL' } } as Parameters<VideoPlayerEvents['statusChange']>[0]);
      fixture.emit('playingChange', { isPlaying: true });
      fixture.emit('timeUpdate', { currentTime: Number.NaN } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
      fixture.emit('timeUpdate', { currentTime: Number.POSITIVE_INFINITY } as Parameters<VideoPlayerEvents['timeUpdate']>[0]);
    }
    const summary = collector.getPlaybackMetadata();
    expect(summary.players).toHaveLength(8);
    expect(summary.droppedPlayers).toBe(4);
    expect(summary.players.every((row) => row.errorObserved && row.maxPositionSeconds === 0 && !row.progressed)).toBe(true);
    expect(JSON.stringify(summary)).not.toContain('PRIVATE_SOURCE_URI_SENTINEL');
    summary.players[0]!.played = false;
    expect(collector.getPlaybackMetadata().players[0]!.played).toBe(true);
    cleanups.forEach((cleanup) => cleanup());
  });
});
