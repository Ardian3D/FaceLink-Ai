import type { VideoPlayer } from 'expo-video';

/** Staging instrumentation only. No media sources, URIs, frames, errors, or player objects retained. */
type PlaybackRow = {
  slot: number; attached: boolean; readyObserved: boolean; errorObserved: boolean;
  playing: boolean; played: boolean; progressed: boolean; pairOverlapObserved: boolean;
  timeUpdates: number; maxPositionSeconds: number; previousPositionSeconds: number;
};
export interface PlaybackMetadata {
  scope: 'staging_actual_host_player_events_only';
  visualAppearanceVerified: false;
  maxPlayersRetained: number;
  droppedPlayers: number;
  bothPlayersPlayedAndProgressed: boolean;
  simultaneousPlayingObserved: boolean;
  players: Omit<PlaybackRow, 'previousPositionSeconds'>[];
}
const MAX_ROWS = 8;
const SATURATE = 1_000_000;
let nextSlot = 0;
let droppedPlayers = 0;
const rows: PlaybackRow[] = [];
const safePosition = (value: number) => Number.isFinite(value) && value >= 0 ? Math.min(value, 86400) : 0;
function observePlaying(row: PlaybackRow, playing: boolean) {
  row.playing = playing;
  row.played ||= playing;
  const newestPair = rows.slice(-2);
  if (newestPair.length === 2 && newestPair.every((item) => item.attached && item.playing)) {
    newestPair.forEach((item) => { item.pairOverlapObserved = true; });
  }
}
export function attachPlaybackMetadata(player: VideoPlayer): () => void {
  // A new VideoBackground mount starts with no attached rows. Retain the just-unmounted
  // pair for the diagnostic read, but never let an older pair contribute to the next one.
  if (rows.every((item) => !item.attached)) {
    rows.length = 0;
    droppedPlayers = 0;
  }
  if (rows.length === MAX_ROWS) { rows.shift(); droppedPlayers = Math.min(SATURATE, droppedPlayers + 1); }
  const position = safePosition(player.currentTime);
  const row: PlaybackRow = {
    slot: nextSlot++ % SATURATE, attached: true, readyObserved: player.status === 'readyToPlay',
    errorObserved: player.status === 'error', playing: false, played: false, progressed: false,
    pairOverlapObserved: false, timeUpdates: 0, maxPositionSeconds: position, previousPositionSeconds: position,
  };
  rows.push(row);
  observePlaying(row, player.playing);
  const subscriptions = [
    player.addListener('statusChange', ({ status }) => {
      row.readyObserved ||= status === 'readyToPlay';
      row.errorObserved ||= status === 'error'; // Never retain the native error object/message.
    }),
    player.addListener('playingChange', ({ isPlaying }) => observePlaying(row, isPlaying)),
    player.addListener('timeUpdate', ({ currentTime }) => {
      const position = safePosition(currentTime);
      row.timeUpdates = Math.min(SATURATE, row.timeUpdates + 1);
      row.progressed ||= row.playing && position > row.previousPositionSeconds;
      row.previousPositionSeconds = position;
      row.maxPositionSeconds = Math.max(row.maxPositionSeconds, position);
    }),
  ];
  let detached = false;
  return () => {
    if (detached) return;
    detached = true;
    subscriptions.forEach((subscription) => subscription.remove());
    row.attached = false;
    row.playing = false;
  };
}
export function getPlaybackMetadata(): PlaybackMetadata {
  const newestPair = rows.slice(-2);
  return {
    scope: 'staging_actual_host_player_events_only' as const,
    visualAppearanceVerified: false,
    maxPlayersRetained: MAX_ROWS,
    droppedPlayers,
    bothPlayersPlayedAndProgressed: newestPair.length === 2 && newestPair.every((row) => row.played && row.progressed),
    simultaneousPlayingObserved: newestPair.length === 2 && newestPair.every((row) => row.pairOverlapObserved),
    players: rows.map(({ previousPositionSeconds: _previous, ...row }) => ({ ...row })),
  };
}
