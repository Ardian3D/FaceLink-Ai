// Konfigurasi ESLint (flat config) untuk aplikasi mobile FaceLink AI.
// Aturan bawaan `eslint-config-expo` dipakai apa adanya; tidak ada aturan yang
// dilonggarkan pada FL-UI01.
const { defineConfig } = require("eslint/config");
const expoConfig = require("eslint-config-expo/flat");

module.exports = defineConfig([
  expoConfig,
  {
    ignores: ["dist/*", "node_modules/*", ".expo/*", "expo-env.d.ts"],
  },
]);
