import React from "react";
import { ScrollView, StyleSheet, View, type ViewStyle } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { colors, layout, spacing } from "@/theme";

/**
 * Kerangka satu layar: latar tema, safe area Android (status bar dan gesture
 * bar), padding konsisten, serta batas lebar konten supaya tampilan tidak
 * melebar pada tablet atau jendela web.
 *
 * `scroll` dipakai untuk layar yang isinya bisa lebih tinggi daripada layar;
 * layar yang mengandalkan tinggi penuh tidak memakainya.
 */
export function Screen({
  children,
  scroll = false,
  center = false,
  contentStyle,
}: {
  children: React.ReactNode;
  scroll?: boolean;
  center?: boolean;
  contentStyle?: ViewStyle;
}) {
  const isi = (
    <View style={[styles.content, center && styles.center, contentStyle]}>{children}</View>
  );

  return (
    <SafeAreaView style={styles.safe} edges={["top", "bottom"]}>
      {scroll ? (
        <ScrollView
          contentContainerStyle={styles.scrollContent}
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}
        >
          {isi}
        </ScrollView>
      ) : (
        isi
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {
    flex: 1,
    backgroundColor: colors.background,
  },
  content: {
    flexGrow: 1,
    width: "100%",
    maxWidth: layout.maxContentWidth,
    alignSelf: "center",
    paddingHorizontal: layout.screenPadding,
    paddingVertical: spacing.xl,
    gap: spacing.lg,
  },
  center: {
    justifyContent: "center",
  },
  scrollContent: {
    flexGrow: 1,
  },
});
