module.exports = {
  dependency: {
    platforms: {
      android: {
        sourceDir: './android',
        packageImportPath: 'import ai.facelink.capturehost.FaceLinkCaptureHostPackage;',
        packageInstance: 'new FaceLinkCaptureHostPackage()',
      },
      ios: null,
    },
  },
};
