import { useEffect } from 'react';
import { useVideoPlayer as useActualVideoPlayer, type VideoPlayer, type VideoSource, type PlayerBuilderOptions } from 'expo-video';
import { attachPlaybackMetadata } from './playback-metadata';

// Metro applies this overlay ONLY to VideoBackground.tsx's expo-video import.
// This module's own import resolves the unmodified SDK, avoiding a resolver loop.
export * from 'expo-video';
export function useVideoPlayer(source: VideoSource, setup?: (player: VideoPlayer) => void, playerBuilderOptions?: PlayerBuilderOptions): VideoPlayer {
  const player = useActualVideoPlayer(source, setup, playerBuilderOptions);
  useEffect(() => attachPlaybackMetadata(player), [player]);
  return player;
}
