package uk.starryheavens.vesper.bothub.ui.theme

import android.os.Build
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext

private val VesperTwilightDarkScheme = darkColorScheme(
    primary = Color(0xFF00E5FF),
    onPrimary = Color(0xFF04141D),
    primaryContainer = Color(0xFF0B2B3C),
    onPrimaryContainer = Color(0xFFB8F4FF),
    secondary = Color(0xFF8CA5FF),
    onSecondary = Color(0xFF0D1530),
    secondaryContainer = Color(0xFF1B2548),
    onSecondaryContainer = Color(0xFFDCE4FF),
    tertiary = Color(0xFFFFBE5C),
    onTertiary = Color(0xFF281900),
    background = Color(0xFF050812),
    onBackground = Color(0xFFEAF0FF),
    surface = Color(0xFF0A0F1F),
    onSurface = Color(0xFFEAF0FF),
    surfaceVariant = Color(0xFF141C33),
    onSurfaceVariant = Color(0xFFA8B5D4),
    outline = Color(0xFF2B385C)
)

@Composable
fun VesperBotHubTheme(
    dynamicColor: Boolean = true,
    content: @Composable () -> Unit
) {
    val context = LocalContext.current
    val colorScheme = if (dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
        dynamicDarkColorScheme(context)
    } else {
        VesperTwilightDarkScheme
    }

    MaterialTheme(
        colorScheme = colorScheme,
        content = content
    )
}
