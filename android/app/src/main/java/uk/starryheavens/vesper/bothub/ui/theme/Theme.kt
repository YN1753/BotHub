package uk.starryheavens.vesper.bothub.ui.theme

import android.os.Build
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext

// Obsidian & Warm Champagne Tactile Glass Scheme (1:1 visual alignment with Vesper Web UI)
private val VesperChampagneDarkScheme = darkColorScheme(
    primary = Color(0xFFF2B880),
    onPrimary = Color(0xFF090C16),
    primaryContainer = Color(0xFF2D221A),
    onPrimaryContainer = Color(0xFFFFE4C4),
    secondary = Color(0xFFA5B4FC),
    onSecondary = Color(0xFF0D142C),
    secondaryContainer = Color(0xFF1A2240),
    onSecondaryContainer = Color(0xFFE2E8FF),
    tertiary = Color(0xFF34D399),
    onTertiary = Color(0xFF062119),
    background = Color(0xFF050811),
    onBackground = Color(0xFFF5F2EB),
    surface = Color(0xFF0B1020),
    onSurface = Color(0xFFF5F2EB),
    surfaceVariant = Color(0xFF131A31),
    onSurfaceVariant = Color(0xFFAAB6CE),
    outline = Color(0xFF2D3856),
    outlineVariant = Color(0xFF1E263E)
)

@Composable
fun VesperBotHubTheme(
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val context = LocalContext.current
    val colorScheme = if (dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
        dynamicDarkColorScheme(context)
    } else {
        VesperChampagneDarkScheme
    }

    MaterialTheme(
        colorScheme = colorScheme,
        content = content
    )
}
