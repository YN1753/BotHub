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
data class BotHubThemeOption(
    val id: String,
    val label: String,
    val swatchAccent: Color,
    val swatchBg: Color,
    val isLightPreview: Boolean = false
)

val BOTHUB_THEME_OPTIONS: List<BotHubThemeOption> = listOf(
    BotHubThemeOption("obsidian", "曜石暖金", Color(0xFFF2B880), Color(0xFF080D1B), isLightPreview = false),
    BotHubThemeOption("ocean", "深海青岚", Color(0xFF38BDF8), Color(0xFF071624), isLightPreview = false),
    BotHubThemeOption("emerald", "松烟翡翠", Color(0xFF34D399), Color(0xFF071813), isLightPreview = false),
    BotHubThemeOption("amethyst", "紫晶星云", Color(0xFFC084FC), Color(0xFF120B24), isLightPreview = false),
    BotHubThemeOption("daylight", "晴雪素白", Color(0xFF2563EB), Color(0xFFF3F6FB), isLightPreview = true),
    BotHubThemeOption("parchment", "宣纸暖茶", Color(0xFFB45309), Color(0xFFF6F0E4), isLightPreview = true),
    BotHubThemeOption("system", "跟随系统", Color(0xFF94A3B8), Color(0xFF1E293B), isLightPreview = false)
)

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

val ObsidianThemeColors = BotHubThemeColors(
    id = "obsidian",
    label = "曜石暖金",
    isLight = false,
    bgBase = Color(0xFF050811),
    auroraPrimary = Color(0x1F6366F1),
    auroraSecondary = Color(0x1AF2B880),
    topBarTop = Color(0xF8060913),
    topBarBottom = Color(0xEB080C1A),
    bottomBarTop = Color(0xF2080D1B),
    bottomBarBottom = Color(0xFA050811),
    sheetBg = Color(0xFF080D1B),
    scrim = Color(0xB802040A),
    cardTop = Color(0xEB131B34),
    cardBottom = Color(0xEB0A0F1F),
    cardUnreadTop = Color(0xF0151F3B),
    cardUnreadBottom = Color(0xF00C1224),
    popoverTop = Color(0xFA16203D),
    popoverBottom = Color(0xFA0C1225),
    inputTop = Color(0xF0070B16),
    inputBottom = Color(0xF00C1224),
    secondaryBtnTop = Color(0xFF182240),
    secondaryBtnBottom = Color(0xFF0E1529),
    userBubbleTop = Color(0xFF3A281C),
    userBubbleBottom = Color(0xFF241812),
    aiBubbleTop = Color(0xF0141D37),
    aiBubbleBottom = Color(0xF00B1021),
    accent = Color(0xFFF2B880),
    accentLight = Color(0xFFFFE2BE),
    accentDeep = Color(0xFFDF9855),
    onAccent = Color(0xFF130E0A),
    textPrimary = Color(0xFFFFF8EE),
    textSecondary = Color(0xFFD8E0F0),
    textMuted = Color(0xFF8C9BB5),
    rimHighlight = Color(0x38FFF8EE),
    rimMid = Color(0x14FFF8EE),
    rimSubtle = Color(0x06FFF8EE),
    divider = Color(0x16FFF8EE),
    codeBlockBg = Color(0xFF050812),
    codeInlineBg = Color(0xFF151E38),
    codeText = Color(0xFFE2E8F0)
)

val OceanThemeColors = BotHubThemeColors(
    id = "ocean",
    label = "深海青岚",
    isLight = false,
    bgBase = Color(0xFF040B13),
    auroraPrimary = Color(0x220EA5E9),
    auroraSecondary = Color(0x182DD4BF),
    topBarTop = Color(0xF8040C15),
    topBarBottom = Color(0xEB071320),
    bottomBarTop = Color(0xF2071422),
    bottomBarBottom = Color(0xFA040B13),
    sheetBg = Color(0xFF071422),
    scrim = Color(0xB802060B),
    cardTop = Color(0xEB0E2034),
    cardBottom = Color(0xEB071320),
    cardUnreadTop = Color(0xF0112840),
    cardUnreadBottom = Color(0xF0091828),
    popoverTop = Color(0xFA10253B),
    popoverBottom = Color(0xFA081523),
    inputTop = Color(0xF0050E18),
    inputBottom = Color(0xF0091726),
    secondaryBtnTop = Color(0xFF122840),
    secondaryBtnBottom = Color(0xFF0A1829),
    userBubbleTop = Color(0xFF0E324B),
    userBubbleBottom = Color(0xFF092033),
    aiBubbleTop = Color(0xF00E2238),
    aiBubbleBottom = Color(0xF0081423),
    accent = Color(0xFF38BDF8),
    accentLight = Color(0xFFBAE6FD),
    accentDeep = Color(0xFF0284C7),
    onAccent = Color(0xFF04131E),
    textPrimary = Color(0xFFF0F9FF),
    textSecondary = Color(0xFFCBD5E1),
    textMuted = Color(0xFF7FA1BE),
    rimHighlight = Color(0x38E0F2FE),
    rimMid = Color(0x14E0F2FE),
    rimSubtle = Color(0x06E0F2FE),
    divider = Color(0x16E0F2FE),
    codeBlockBg = Color(0xFF030911),
    codeInlineBg = Color(0xFF0E2439),
    codeText = Color(0xFFE0F2FE)
)

val EmeraldThemeColors = BotHubThemeColors(
    id = "emerald",
    label = "松烟翡翠",
    isLight = false,
    bgBase = Color(0xFF040D0A),
    auroraPrimary = Color(0x2010B981),
    auroraSecondary = Color(0x1638BDF8),
    topBarTop = Color(0xF8050E0B),
    topBarBottom = Color(0xEB081612),
    bottomBarTop = Color(0xF2081713),
    bottomBarBottom = Color(0xFA040D0A),
    sheetBg = Color(0xFF071612),
    scrim = Color(0xB8020806),
    cardTop = Color(0xEB0E241E),
    cardBottom = Color(0xEB081511),
    cardUnreadTop = Color(0xF0112D25),
    cardUnreadBottom = Color(0xF00A1915),
    popoverTop = Color(0xFA112A23),
    popoverBottom = Color(0xFA081713),
    inputTop = Color(0xF0050F0C),
    inputBottom = Color(0xF0091914),
    secondaryBtnTop = Color(0xFF122C24),
    secondaryBtnBottom = Color(0xFF0A1A15),
    userBubbleTop = Color(0xFF10382B),
    userBubbleBottom = Color(0xFF0A241C),
    aiBubbleTop = Color(0xF00E241E),
    aiBubbleBottom = Color(0xF0081511),
    accent = Color(0xFF34D399),
    accentLight = Color(0xFFA7F3D0),
    accentDeep = Color(0xFF059669),
    onAccent = Color(0xFF041A12),
    textPrimary = Color(0xFFECFDF5),
    textSecondary = Color(0xFFD1FAE5),
    textMuted = Color(0xFF84A99C),
    rimHighlight = Color(0x36D1FAE5),
    rimMid = Color(0x14D1FAE5),
    rimSubtle = Color(0x06D1FAE5),
    divider = Color(0x16D1FAE5),
    codeBlockBg = Color(0xFF030A08),
    codeInlineBg = Color(0xFF0F261F),
    codeText = Color(0xFFD1FAE5)
)

val AmethystThemeColors = BotHubThemeColors(
    id = "amethyst",
    label = "紫晶星云",
    isLight = false,
    bgBase = Color(0xFF090613),
    auroraPrimary = Color(0x24A855F7),
    auroraSecondary = Color(0x18F472B6),
    topBarTop = Color(0xF80A0716),
    topBarBottom = Color(0xEB100B22),
    bottomBarTop = Color(0xF2110B23),
    bottomBarBottom = Color(0xFA090613),
    sheetBg = Color(0xFF110B22),
    scrim = Color(0xB804020A),
    cardTop = Color(0xEB1C1335),
    cardBottom = Color(0xEB0F0A1F),
    cardUnreadTop = Color(0xF0231742),
    cardUnreadBottom = Color(0xF0130C26),
    popoverTop = Color(0xFA21163E),
    popoverBottom = Color(0xFA120B24),
    inputTop = Color(0xF00B0718),
    inputBottom = Color(0xF0130D26),
    secondaryBtnTop = Color(0xFF231740),
    secondaryBtnBottom = Color(0xFF140D28),
    userBubbleTop = Color(0xFF331D54),
    userBubbleBottom = Color(0xFF201138),
    aiBubbleTop = Color(0xF01B1234),
    aiBubbleBottom = Color(0xF00F0A1E),
    accent = Color(0xFFC084FC),
    accentLight = Color(0xFFE9D5FF),
    accentDeep = Color(0xFF9333EA),
    onAccent = Color(0xFF150726),
    textPrimary = Color(0xFFFAF5FF),
    textSecondary = Color(0xFFE2D9F3),
    textMuted = Color(0xFF9D8EC0),
    rimHighlight = Color(0x38F3E8FF),
    rimMid = Color(0x14F3E8FF),
    rimSubtle = Color(0x06F3E8FF),
    divider = Color(0x16F3E8FF),
    codeBlockBg = Color(0xFF070410),
    codeInlineBg = Color(0xFF1E1438),
    codeText = Color(0xFFEDE9FE)
)

val DaylightThemeColors = BotHubThemeColors(
    id = "daylight",
    label = "晴雪素白",
    isLight = true,
    bgBase = Color(0xFFF2F5FA),
    auroraPrimary = Color(0x143B82F6),
    auroraSecondary = Color(0x120EA5E9),
    topBarTop = Color(0xFAFFFFFF),
    topBarBottom = Color(0xF5F6F8FC),
    bottomBarTop = Color(0xFAFFFFFF),
    bottomBarBottom = Color(0xFFF8FAFC),
    sheetBg = Color(0xFFF8FAFC),
    scrim = Color(0x660F172A),
    cardTop = Color(0xFFFFFFFF),
    cardBottom = Color(0xFFF8FAFD),
    cardUnreadTop = Color(0xFFFFFFFF),
    cardUnreadBottom = Color(0xFFEFF6FF),
    popoverTop = Color(0xFFFFFFFF),
    popoverBottom = Color(0xFFF5F8FC),
    inputTop = Color(0xFFEDF2F8),
    inputBottom = Color(0xFFF4F7FB),
    secondaryBtnTop = Color(0xFFFFFFFF),
    secondaryBtnBottom = Color(0xFFEEF3F9),
    userBubbleTop = Color(0xFFDBEAFE),
    userBubbleBottom = Color(0xFFBFDBFE),
    aiBubbleTop = Color(0xFFFFFFFF),
    aiBubbleBottom = Color(0xFFF8FAFC),
    accent = Color(0xFF2563EB),
    accentLight = Color(0xFF1D4ED8),
    accentDeep = Color(0xFF1E40AF),
    onAccent = Color(0xFFFFFFFF),
    textPrimary = Color(0xFF0F172A),
    textSecondary = Color(0xFF334155),
    textMuted = Color(0xFF64748B),
    rimHighlight = Color(0x220F172A),
    rimMid = Color(0x140F172A),
    rimSubtle = Color(0x0A0F172A),
    divider = Color(0x180F172A),
    codeBlockBg = Color(0xFFEEF2F7),
    codeInlineBg = Color(0xFFE2E8F0),
    codeText = Color(0xFF0F172A)
)

val ParchmentThemeColors = BotHubThemeColors(
    id = "parchment",
    label = "宣纸暖茶",
    isLight = true,
    bgBase = Color(0xFFF4EEE2),
    auroraPrimary = Color(0x14D97706),
    auroraSecondary = Color(0x12B45309),
    topBarTop = Color(0xFAFCF9F2),
    topBarBottom = Color(0xF5F6F0E4),
    bottomBarTop = Color(0xFAFCF9F2),
    bottomBarBottom = Color(0xFFF6F0E4),
    sheetBg = Color(0xFFFAF6EC),
    scrim = Color(0x66292018),
    cardTop = Color(0xFFFFFDF8),
    cardBottom = Color(0xFFF8F3E8),
    cardUnreadTop = Color(0xFFFFFDF9),
    cardUnreadBottom = Color(0xFFFEF3C7),
    popoverTop = Color(0xFFFFFDF8),
    popoverBottom = Color(0xFFF7F1E4),
    inputTop = Color(0xFFEAE2D2),
    inputBottom = Color(0xFFF1EADC),
    secondaryBtnTop = Color(0xFFFFFDF8),
    secondaryBtnBottom = Color(0xFFEDE5D5),
    userBubbleTop = Color(0xFFFDE68A),
    userBubbleBottom = Color(0xFFFCD34D),
    aiBubbleTop = Color(0xFFFFFDF9),
    aiBubbleBottom = Color(0xFFF8F3E7),
    accent = Color(0xFFB45309),
    accentLight = Color(0xFF92400E),
    accentDeep = Color(0xFF78350F),
    onAccent = Color(0xFFFFFBEB),
    textPrimary = Color(0xFF1F1A14),
    textSecondary = Color(0xFF4A3F33),
    textMuted = Color(0xFF786B59),
    rimHighlight = Color(0x243E2F1C),
    rimMid = Color(0x143E2F1C),
    rimSubtle = Color(0x0A3E2F1C),
    divider = Color(0x1A3E2F1C),
    codeBlockBg = Color(0xFFEBE3D3),
    codeInlineBg = Color(0xFFE2D8C3),
    codeText = Color(0xFF292018)
)

fun resolveBotHubThemeColors(themeId: String, systemDark: Boolean): BotHubThemeColors {
    return when (themeId.trim().lowercase()) {
        "ocean" -> OceanThemeColors
        "emerald" -> EmeraldThemeColors
        "amethyst" -> AmethystThemeColors
        "daylight" -> DaylightThemeColors
        "parchment" -> ParchmentThemeColors
        "system" -> if (systemDark) ObsidianThemeColors else DaylightThemeColors
        else -> ObsidianThemeColors
    }
}

val LocalBotHubColors = staticCompositionLocalOf { ObsidianThemeColors }

val BotHubThemeTokens: BotHubThemeColors
    @Composable
    @ReadOnlyComposable
    get() = LocalBotHubColors.current

@Composable
fun VesperBotHubTheme(
    themeId: String = "obsidian",
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
            onPrimaryContainer = colors.textPrimary,
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
            onPrimaryContainer = colors.accentLight,
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
