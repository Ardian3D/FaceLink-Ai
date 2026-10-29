
// Staging-only observer: original VideoBackground source and SDK implementation stay unchanged.
// Only its expo-video import is redirected; the observer's import resolves the actual SDK.
const cam03VideoResolver = config.resolver.resolveRequest;
config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName === 'expo-video' && context.originModulePath &&
      path.resolve(context.originModulePath) === path.join(__dirname, 'src/components/VideoBackground.tsx')) {
    return context.resolveRequest(context, path.join(__dirname, 'src/cam03/video-observer.ts'), platform);
  }
  return (cam03VideoResolver ?? context.resolveRequest)(context, moduleName, platform);
};
