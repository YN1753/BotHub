package uk.starryheavens.vesper.bothub.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

@Immutable
data class BotHubColorModeOption(
    val id: String,
    val label: String
)

val BOTHUB_COLOR_MODE_OPTIONS: List<BotHubColorModeOption> = listOf(
    BotHubColorModeOption("dark", "深色"),
    BotHubColorModeOption("light", "浅色"),
    BotHubColorModeOption("system", "跟随系统")
)

@Immutable
data class BotHubAccentOption(
    val id: String,
    val label: String,
    val darkSwatch: Color,
    val lightSwatch: Color
)

val BOTHUB_ACCENT_OPTIONS: List<BotHubAccentOption> = listOf(
    BotHubAccentOption("blue", "经典蓝", Color(0xFF4F8FF7), Color(0xFF2563EB)),
    BotHubAccentOption("amber", "琥珀金", Color(0xFFE5A45C), Color(0xFFB45309)),
    BotHubAccentOption("cyan", "青岚", Color(0xFF22C3DC), Color(0xFF0284C7)),
    BotHubAccentOption("emerald", "翠绿", Color(0xFF10B981), Color(0xFF059669)),
    BotHubAccentOption("violet", "紫晶", Color(0xFF9370F8), Color(0xFF7C3AED)),
    BotHubAccentOption("mono", "曜石黑白", Color(0xFFF3F4F6), Color(0xFF111827))
)

private val VALID_COLOR_MODES = setOf("dark", "light", "system")
private val VALID_ACCENTS = setOf("blue", "amber", "cyan", "emerald", "violet", "mono")

fun parseThemePreference(raw: String): Pair<String, String> {
    val clean = raw.trim().lowercase()
    if (clean.contains(':')) {
        val parts = clean.split(':', limit = 2)
        val rawMode = parts[0].trim()
        val rawAccent = parts.getOrElse(1) { "" }.trim()
        val safeMode = if (rawMode in VALID_COLOR_MODES) rawMode else "dark"
        val safeAccent = if (rawAccent in VALID_ACCENTS) rawAccent else "blue"
        return safeMode to safeAccent
    }
    return when (clean) {
        "obsidian" -> "dark" to "amber"
        "ocean" -> "dark" to "cyan"
        "emerald" -> "dark" to "emerald"
        "amethyst" -> "dark" to "violet"
        "daylight" -> "light" to "blue"
        "parchment" -> "light" to "amber"
        "system" -> "system" to "blue"
        "dark" -> "dark" to "blue"
        "light" -> "light" to "blue"
        else -> "dark" to "blue"
    }
}

fun encodeThemePreference(mode: String, accent: String): String {
    val cleanMode = mode.trim().lowercase()
    val cleanAccent = accent.trim().lowercase()
    val safeMode = if (cleanMode in VALID_COLOR_MODES) cleanMode else "dark"
    val safeAccent = if (cleanAccent in VALID_ACCENTS) cleanAccent else "blue"
    return "$safeMode:$safeAccent"
}

/**
 * Semantic design tokens for Vesper BotHub.
 * Layered neutral surfaces + crisp structural borders + purposeful accent and status roles.
 */
@Immutable
data class BotHubThemeColors(
    val id: String,
    val label: String,
    val isLight: Boolean,
    // Surface elevation hierarchy
    val bgCanvas: Color,
    val surfaceBar: Color,
    val surfaceCard: Color,
    val surfaceCardElevated: Color,
    val surfaceSheet: Color,
    val surfacePopover: Color,
    val surfaceInput: Color,
    val surfaceControl: Color,
    val scrim: Color,
    // Chat bubbles
    val userBubble: Color,
    val userBubbleText: Color,
    val aiBubble: Color,
    // Brand accent roles
    val accent: Color,
    val accentSoft: Color,
    val accentMuted: Color,
    val accentForeground: Color,
    val onAccent: Color,
    // Semantic status palette
    val success: Color,
    val warning: Color,
    val danger: Color,
    val info: Color,
    // Typography hierarchy
    val textPrimary: Color,
    val textSecondary: Color,
    val textMuted: Color,
    // Outlines and dividers
    val border: Color,
    val borderStrong: Color,
    val divider: Color,
    // Code & technical blocks
    val codeBlockBg: Color,
    val codeInlineBg: Color,
    val codeText: Color
)

private data class AccentPalette(
    val accent: Color,
    val accentSoft: Color,
    val accentMuted: Color,
    val accentForeground: Color,
    val onAccent: Color
)

private fun resolveAccentPalette(accentId: String, isLight: Boolean): AccentPalette {
    return if (isLight) {
        when (accentId) {
            "amber" -> AccentPalette(
                accent = Color(0xFFB45309),
                accentSoft = Color(0x16B45309),
                accentMuted = Color(0x47B45309),
                accentForeground = Color(0xFF92400E),
                onAccent = Color(0xFFFFFFFF)
            )
            "cyan" -> AccentPalette(
                accent = Color(0xFF0284C7),
                accentSoft = Color(0x160284C7),
                accentMuted = Color(0x470284C7),
                accentForeground = Color(0xFF0369A1),
                onAccent = Color(0xFFFFFFFF)
            )
            "emerald" -> AccentPalette(
                accent = Color(0xFF059669),
                accentSoft = Color(0x16059669),
                accentMuted = Color(0x47059669),
                accentForeground = Color(0xFF047857),
                onAccent = Color(0xFFFFFFFF)
            )
            "violet" -> AccentPalette(
                accent = Color(0xFF7C3AED),
                accentSoft = Color(0x167C3AED),
                accentMuted = Color(0x477C3AED),
                accentForeground = Color(0xFF6D28D9),
                onAccent = Color(0xFFFFFFFF)
            )
            "mono" -> AccentPalette(
                accent = Color(0xFF111827),
                accentSoft = Color(0x12111827),
                accentMuted = Color(0x38111827),
                accentForeground = Color(0xFF111827),
                onAccent = Color(0xFFFFFFFF)
            )
            else -> AccentPalette(
                accent = Color(0xFF2563EB),
                accentSoft = Color(0x162563EB),
                accentMuted = Color(0x472563EB),
                accentForeground = Color(0xFF1D4ED8),
                onAccent = Color(0xFFFFFFFF)
            )
        }
    } else {
        when (accentId) {
            "amber" -> AccentPalette(
                accent = Color(0xFFE5A45C),
                accentSoft = Color(0x1FE5A45C),
                accentMuted = Color(0x52E5A45C),
                accentForeground = Color(0xFFF6C88F),
                onAccent = Color(0xFF14100C)
            )
            "cyan" -> AccentPalette(
                accent = Color(0xFF22C3DC),
                accentSoft = Color(0x1F22C3DC),
                accentMuted = Color(0x5222C3DC),
                accentForeground = Color(0xFF67E8F9),
                onAccent = Color(0xFF051317)
            )
            "emerald" -> AccentPalette(
                accent = Color(0xFF10B981),
                accentSoft = Color(0x1F10B981),
                accentMuted = Color(0x5210B981),
                accentForeground = Color(0xFF6EE7B7),
                onAccent = Color(0xFF041610)
            )
            "violet" -> AccentPalette(
                accent = Color(0xFF9370F8),
                accentSoft = Color(0x219370F8),
                accentMuted = Color(0x549370F8),
                accentForeground = Color(0xFFC4B5FD),
                onAccent = Color(0xFFFFFFFF)
            )
            "mono" -> AccentPalette(
                accent = Color(0xFFF3F4F6),
                accentSoft = Color(0x1AF3F4F6),
                accentMuted = Color(0x40F3F4F6),
                accentForeground = Color(0xFFF3F4F6),
                onAccent = Color(0xFF090A0D)
            )
            else -> AccentPalette(
                accent = Color(0xFF4F8FF7),
                accentSoft = Color(0x1F4F8FF7),
                accentMuted = Color(0x524F8FF7),
                accentForeground = Color(0xFF93C5FD),
                onAccent = Color(0xFFFFFFFF)
            )
        }
    }
}

fun resolveBotHubThemeColors(themeId: String, systemDark: Boolean): BotHubThemeColors {
    val (mode, accentId) = parseThemePreference(themeId)
    val isLight = when (mode) {
        "light" -> true
        "dark" -> false
        else -> !systemDark
    }
    val accent = resolveAccentPalette(accentId, isLight)
    val modeLabel = BOTHUB_COLOR_MODE_OPTIONS.find { it.id == mode }?.label
        ?: if (isLight) "浅色" else "深色"
    val accentLabel = BOTHUB_ACCENT_OPTIONS.find { it.id == accentId }?.label ?: "经典蓝"
    val resolvedId = "${if (isLight) "light" else "dark"}:$accentId"
    val resolvedLabel = "$modeLabel · $accentLabel"

    return if (isLight) {
        BotHubThemeColors(
            id = resolvedId,
            label = resolvedLabel,
            isLight = true,
            bgCanvas = Color(0xFFF5F6F8),
            surfaceBar = Color(0xFFFFFFFF),
            surfaceCard = Color(0xFFFFFFFF),
            surfaceCardElevated = Color(0xFFF2F5FA),
            surfaceSheet = Color(0xFFFFFFFF),
            surfacePopover = Color(0xFFFFFFFF),
            surfaceInput = Color(0xFFECEEF2),
            surfaceControl = Color(0xFFF8F9FB),
            scrim = Color(0x520F172A),
            userBubble = accent.accent,
            userBubbleText = accent.onAccent,
            aiBubble = Color(0xFFFFFFFF),
            accent = accent.accent,
            accentSoft = accent.accentSoft,
            accentMuted = accent.accentMuted,
            accentForeground = accent.accentForeground,
            onAccent = accent.onAccent,
            success = Color(0xFF059669),
            warning = Color(0xFFD97706),
            danger = Color(0xFFDC2626),
            info = Color(0xFF0284C7),
            textPrimary = Color(0xFF0F172A),
            textSecondary = Color(0xFF475569),
            textMuted = Color(0xFF64748B),
            border = Color(0xFFE2E6EE),
            borderStrong = Color(0xFFCBD2DF),
            divider = Color(0xFFEAEDF2),
            codeBlockBg = Color(0xFFF1F3F7),
            codeInlineBg = Color(0xFFE6EAF0),
            codeText = Color(0xFF1E293B)
        )
    } else {
        BotHubThemeColors(
            id = resolvedId,
            label = resolvedLabel,
            isLight = false,
            bgCanvas = Color(0xFF090A0D),
            surfaceBar = Color(0xFF0F1116),
            surfaceCard = Color(0xFF14171F),
            surfaceCardElevated = Color(0xFF1B1F2A),
            surfaceSheet = Color(0xFF11141B),
            surfacePopover = Color(0xFF1B1F29),
            surfaceInput = Color(0xFF0E1016),
            surfaceControl = Color(0xFF191D26),
            scrim = Color(0xA6000000),
            userBubble = accent.accent,
            userBubbleText = accent.onAccent,
            aiBubble = Color(0xFF14171F),
            accent = accent.accent,
            accentSoft = accent.accentSoft,
            accentMuted = accent.accentMuted,
            accentForeground = accent.accentForeground,
            onAccent = accent.onAccent,
            success = Color(0xFF34D399),
            warning = Color(0xFFFBBF24),
            danger = Color(0xFFFB7185),
            info = Color(0xFF38BDF8),
            textPrimary = Color(0xFFF3F4F6),
            textSecondary = Color(0xFFA6B0C0),
            textMuted = Color(0xFF727C8E),
            border = Color(0x1EFFFFFF),
            borderStrong = Color(0x30FFFFFF),
            divider = Color(0x14FFFFFF),
            codeBlockBg = Color(0xFF0D0F14),
            codeInlineBg = Color(0xFF1E232E),
            codeText = Color(0xFFE2E8F0)
        )
    }
}

private val DefaultDarkThemeColors: BotHubThemeColors = resolveBotHubThemeColors("dark:blue", true)

val LocalBotHubColors = staticCompositionLocalOf { DefaultDarkThemeColors }

val BotHubThemeTokens: BotHubThemeColors
    @Composable
    @ReadOnlyComposable
    get() = LocalBotHubColors.current

@Composable
fun VesperBotHubTheme(
    themeId: String = "dark:blue",
    @Suppress("UNUSED_PARAMETER") dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val systemDark = isSystemInDarkTheme()
    val colors = resolveBotHubThemeColors(themeId, systemDark)

    val m3Scheme = if (colors.isLight) {
        lightColorScheme(
            primary = colors.accent,
            onPrimary = colors.onAccent,
            primaryContainer = colors.accentSoft,
            onPrimaryContainer = colors.accentForeground,
            secondary = colors.info,
            onSecondary = Color.White,
            secondaryContainer = colors.surfaceInput,
            onSecondaryContainer = colors.textPrimary,
            tertiary = colors.success,
            onTertiary = Color.White,
            error = colors.danger,
            onError = Color.White,
            background = colors.bgCanvas,
            onBackground = colors.textPrimary,
            surface = colors.surfaceSheet,
            onSurface = colors.textPrimary,
            surfaceVariant = colors.surfaceCard,
            onSurfaceVariant = colors.textSecondary,
            surfaceContainer = colors.surfaceCard,
            surfaceContainerHigh = colors.surfacePopover,
            surfaceContainerHighest = colors.surfaceCardElevated,
            surfaceContainerLow = colors.surfaceBar,
            surfaceContainerLowest = colors.bgCanvas,
            inverseSurface = colors.surfacePopover,
            inverseOnSurface = colors.textPrimary,
            inversePrimary = colors.accent,
            outline = colors.borderStrong,
            outlineVariant = colors.border,
            scrim = colors.scrim
        )
    } else {
        darkColorScheme(
            primary = colors.accent,
            onPrimary = colors.onAccent,
            primaryContainer = colors.accentSoft,
            onPrimaryContainer = colors.accentForeground,
            secondary = colors.info,
            onSecondary = Color(0xFF061622),
            secondaryContainer = colors.surfaceControl,
            onSecondaryContainer = colors.textPrimary,
            tertiary = colors.success,
            onTertiary = Color(0xFF062119),
            error = colors.danger,
            onError = Color(0xFF28070D),
            background = colors.bgCanvas,
            onBackground = colors.textPrimary,
            surface = colors.surfaceSheet,
            onSurface = colors.textPrimary,
            surfaceVariant = colors.surfaceCard,
            onSurfaceVariant = colors.textSecondary,
            surfaceContainer = colors.surfaceCard,
            surfaceContainerHigh = colors.surfacePopover,
            surfaceContainerHighest = colors.surfaceCardElevated,
            surfaceContainerLow = colors.surfaceBar,
            surfaceContainerLowest = colors.bgCanvas,
            inverseSurface = colors.surfacePopover,
            inverseOnSurface = colors.textPrimary,
            inversePrimary = colors.accent,
            outline = colors.borderStrong,
            outlineVariant = colors.border,
            scrim = colors.scrim
        )
    }

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as? Activity)?.window
            if (window != null) {
                val controller = WindowCompat.getInsetsController(window, view)
                controller.isAppearanceLightStatusBars = colors.isLight
                controller.isAppearanceLightNavigationBars = colors.isLight
            }
        }
    }

    CompositionLocalProvider(LocalBotHubColors provides colors) {
        MaterialTheme(
            colorScheme = m3Scheme,
            content = content
        )
    }
}
