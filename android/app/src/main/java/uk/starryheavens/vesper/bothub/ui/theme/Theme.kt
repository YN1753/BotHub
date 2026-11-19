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
    BotHubAccentOption("blue", "经典蓝", Color(0xFF3B82F6), Color(0xFF2563EB)),
    BotHubAccentOption("amber", "琥珀金", Color(0xFFE5A45C), Color(0xFFB45309)),
    BotHubAccentOption("cyan", "青岚", Color(0xFF06B6D4), Color(0xFF0284C7)),
    BotHubAccentOption("emerald", "翠绿", Color(0xFF10B981), Color(0xFF059669)),
    BotHubAccentOption("violet", "紫晶", Color(0xFF8B5CF6), Color(0xFF7C3AED)),
    BotHubAccentOption("mono", "曜石黑白", Color(0xFFF4F5F7), Color(0xFF111827))
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

@Immutable
data class BotHubThemeColors(
    val id: String,
    val label: String,
    val isLight: Boolean,
    val bgBase: Color,
    val auroraPrimary: Color,
    val auroraSecondary: Color,
    val topBarTop: Color,
    val topBarBottom: Color,
    val bottomBarTop: Color,
    val bottomBarBottom: Color,
    val sheetBg: Color,
    val scrim: Color,
    val cardTop: Color,
    val cardBottom: Color,
    val cardUnreadTop: Color,
    val cardUnreadBottom: Color,
    val popoverTop: Color,
    val popoverBottom: Color,
    val inputTop: Color,
    val inputBottom: Color,
    val secondaryBtnTop: Color,
    val secondaryBtnBottom: Color,
    val userBubbleTop: Color,
    val userBubbleBottom: Color,
    val userBubbleText: Color,
    val aiBubbleTop: Color,
    val aiBubbleBottom: Color,
    val accent: Color,
    val accentLight: Color,
    val accentDeep: Color,
    val onAccent: Color,
    val textPrimary: Color,
    val textSecondary: Color,
    val textMuted: Color,
    val rimHighlight: Color,
    val rimMid: Color,
    val rimSubtle: Color,
    val divider: Color,
    val codeBlockBg: Color,
    val codeInlineBg: Color,
    val codeText: Color
)

private data class AccentTokens(
    val accent: Color,
    val accentLight: Color,
    val accentDeep: Color,
    val onAccent: Color
)

private fun resolveAccentTokens(accentId: String, isLight: Boolean): AccentTokens {
    return if (isLight) {
        when (accentId) {
            "amber" -> AccentTokens(
                accent = Color(0xFFB45309),
                accentLight = Color(0xFFD97706),
                accentDeep = Color(0xFF92400E),
                onAccent = Color(0xFFFFFFFF)
            )
            "cyan" -> AccentTokens(
                accent = Color(0xFF0284C7),
                accentLight = Color(0xFF0EA5E9),
                accentDeep = Color(0xFF0369A1),
                onAccent = Color(0xFFFFFFFF)
            )
            "emerald" -> AccentTokens(
                accent = Color(0xFF059669),
                accentLight = Color(0xFF10B981),
                accentDeep = Color(0xFF047857),
                onAccent = Color(0xFFFFFFFF)
            )
            "violet" -> AccentTokens(
                accent = Color(0xFF7C3AED),
                accentLight = Color(0xFF8B5CF6),
                accentDeep = Color(0xFF6D28D9),
                onAccent = Color(0xFFFFFFFF)
            )
            "mono" -> AccentTokens(
                accent = Color(0xFF111827),
                accentLight = Color(0xFF374151),
                accentDeep = Color(0xFF030712),
                onAccent = Color(0xFFFFFFFF)
            )
            else -> AccentTokens(
                accent = Color(0xFF2563EB),
                accentLight = Color(0xFF3B82F6),
                accentDeep = Color(0xFF1D4ED8),
                onAccent = Color(0xFFFFFFFF)
            )
        }
    } else {
        when (accentId) {
            "amber" -> AccentTokens(
                accent = Color(0xFFE5A45C),
                accentLight = Color(0xFFFDE0B6),
                accentDeep = Color(0xFFC8853C),
                onAccent = Color(0xFF14110E)
            )
            "cyan" -> AccentTokens(
                accent = Color(0xFF06B6D4),
                accentLight = Color(0xFF67E8F9),
                accentDeep = Color(0xFF0891B2),
                onAccent = Color(0xFF051317)
            )
            "emerald" -> AccentTokens(
                accent = Color(0xFF10B981),
                accentLight = Color(0xFF6EE7B7),
                accentDeep = Color(0xFF059669),
                onAccent = Color(0xFF041610)
            )
            "violet" -> AccentTokens(
                accent = Color(0xFF8B5CF6),
                accentLight = Color(0xFFC4B5FD),
                accentDeep = Color(0xFF7C3AED),
                onAccent = Color(0xFFFFFFFF)
            )
            "mono" -> AccentTokens(
                accent = Color(0xFFF4F5F7),
                accentLight = Color(0xFFFFFFFF),
                accentDeep = Color(0xFFD1D5DB),
                onAccent = Color(0xFF0B0C0E)
            )
            else -> AccentTokens(
                accent = Color(0xFF3B82F6),
                accentLight = Color(0xFF93C5FD),
                accentDeep = Color(0xFF2563EB),
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
    val accentTokens = resolveAccentTokens(accentId, isLight)
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
            bgBase = Color(0xFFF6F7F9),
            auroraPrimary = Color.Transparent,
            auroraSecondary = Color.Transparent,
            topBarTop = Color(0xFFFFFFFF),
            topBarBottom = Color(0xFFFFFFFF),
            bottomBarTop = Color(0xFFFFFFFF),
            bottomBarBottom = Color(0xFFFFFFFF),
            sheetBg = Color(0xFFFFFFFF),
            scrim = Color(0x5C0F172A),
            cardTop = Color(0xFFFFFFFF),
            cardBottom = Color(0xFFFFFFFF),
            cardUnreadTop = Color(0xFFF0F4FF),
            cardUnreadBottom = Color(0xFFF0F4FF),
            popoverTop = Color(0xFFFFFFFF),
            popoverBottom = Color(0xFFFFFFFF),
            inputTop = Color(0xFFF3F4F6),
            inputBottom = Color(0xFFF3F4F6),
            secondaryBtnTop = Color(0xFFFFFFFF),
            secondaryBtnBottom = Color(0xFFFFFFFF),
            userBubbleTop = accentTokens.accent,
            userBubbleBottom = accentTokens.accent,
            userBubbleText = accentTokens.onAccent,
            aiBubbleTop = Color(0xFFFFFFFF),
            aiBubbleBottom = Color(0xFFFFFFFF),
            accent = accentTokens.accent,
            accentLight = accentTokens.accentLight,
            accentDeep = accentTokens.accentDeep,
            onAccent = accentTokens.onAccent,
            textPrimary = Color(0xFF111827),
            textSecondary = Color(0xFF4B5563),
            textMuted = Color(0xFF6B7280),
            rimHighlight = Color(0xFFE4E7EC),
            rimMid = Color(0xFFE4E7EC),
            rimSubtle = Color(0xFFEAECF0),
            divider = Color(0xFFE4E7EC),
            codeBlockBg = Color(0xFFF6F7F9),
            codeInlineBg = Color(0xFFEEF0F4),
            codeText = Color(0xFF111827)
        )
    } else {
        BotHubThemeColors(
            id = resolvedId,
            label = resolvedLabel,
            isLight = false,
            bgBase = Color(0xFF0B0C0E),
            auroraPrimary = Color.Transparent,
            auroraSecondary = Color.Transparent,
            topBarTop = Color(0xFF111317),
            topBarBottom = Color(0xFF111317),
            bottomBarTop = Color(0xFF111317),
            bottomBarBottom = Color(0xFF111317),
            sheetBg = Color(0xFF111317),
            scrim = Color(0x99000000),
            cardTop = Color(0xFF1A1D24),
            cardBottom = Color(0xFF1A1D24),
            cardUnreadTop = Color(0xFF20242D),
            cardUnreadBottom = Color(0xFF20242D),
            popoverTop = Color(0xFF1E222B),
            popoverBottom = Color(0xFF1E222B),
            inputTop = Color(0xFF111317),
            inputBottom = Color(0xFF111317),
            secondaryBtnTop = Color(0xFF1A1D24),
            secondaryBtnBottom = Color(0xFF1A1D24),
            userBubbleTop = accentTokens.accent,
            userBubbleBottom = accentTokens.accent,
            userBubbleText = accentTokens.onAccent,
            aiBubbleTop = Color(0xFF1A1D24),
            aiBubbleBottom = Color(0xFF1A1D24),
            accent = accentTokens.accent,
            accentLight = accentTokens.accentLight,
            accentDeep = accentTokens.accentDeep,
            onAccent = accentTokens.onAccent,
            textPrimary = Color(0xFFF4F5F7),
            textSecondary = Color(0xFFB4BCC8),
            textMuted = Color(0xFF7D8594),
            rimHighlight = Color(0x24FFFFFF),
            rimMid = Color(0x18FFFFFF),
            rimSubtle = Color(0x14FFFFFF),
            divider = Color(0x18FFFFFF),
            codeBlockBg = Color(0xFF0E1014),
            codeInlineBg = Color(0xFF222630),
            codeText = Color(0xFFE5E7EB)
        )
    }
}

val ObsidianThemeColors: BotHubThemeColors = resolveBotHubThemeColors("dark:amber", true)

val LocalBotHubColors = staticCompositionLocalOf { ObsidianThemeColors }

val BotHubThemeTokens: BotHubThemeColors
    @Composable
    @ReadOnlyComposable
    get() = LocalBotHubColors.current

@Composable
fun VesperBotHubTheme(
    themeId: String = "dark:blue",
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val systemDark = isSystemInDarkTheme()
    val colors = resolveBotHubThemeColors(themeId, systemDark)

    val m3Scheme = if (colors.isLight) {
        lightColorScheme(
            primary = colors.accent,
            onPrimary = colors.onAccent,
            primaryContainer = colors.userBubbleTop,
            onPrimaryContainer = colors.userBubbleText,
            secondary = colors.accentDeep,
            onSecondary = colors.onAccent,
            secondaryContainer = colors.inputTop,
            onSecondaryContainer = colors.textPrimary,
            background = colors.bgBase,
            onBackground = colors.textPrimary,
            surface = colors.sheetBg,
            onSurface = colors.textPrimary,
            surfaceVariant = colors.cardBottom,
            onSurfaceVariant = colors.textSecondary,
            surfaceContainer = colors.sheetBg,
            surfaceContainerHigh = colors.popoverTop,
            surfaceContainerHighest = colors.popoverTop,
            surfaceContainerLow = colors.cardBottom,
            surfaceContainerLowest = colors.bgBase,
            inverseSurface = colors.popoverTop,
            inverseOnSurface = colors.textPrimary,
            inversePrimary = colors.accent,
            outline = colors.rimHighlight,
            outlineVariant = colors.divider,
            scrim = colors.scrim
        )
    } else {
        darkColorScheme(
            primary = colors.accent,
            onPrimary = colors.onAccent,
            primaryContainer = colors.userBubbleTop,
            onPrimaryContainer = colors.userBubbleText,
            secondary = Color(0xFFA5B4FC),
            onSecondary = Color(0xFF0D142C),
            secondaryContainer = colors.secondaryBtnTop,
            onSecondaryContainer = colors.textPrimary,
            tertiary = Color(0xFF34D399),
            onTertiary = Color(0xFF062119),
            background = colors.bgBase,
            onBackground = colors.textPrimary,
            surface = colors.sheetBg,
            onSurface = colors.textPrimary,
            surfaceVariant = colors.cardTop,
            onSurfaceVariant = colors.textSecondary,
            surfaceContainer = colors.sheetBg,
            surfaceContainerHigh = colors.popoverTop,
            surfaceContainerHighest = colors.popoverTop,
            surfaceContainerLow = colors.cardBottom,
            surfaceContainerLowest = colors.bgBase,
            inverseSurface = colors.popoverTop,
            inverseOnSurface = colors.textPrimary,
            inversePrimary = colors.accent,
            outline = colors.rimHighlight,
            outlineVariant = colors.divider,
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
