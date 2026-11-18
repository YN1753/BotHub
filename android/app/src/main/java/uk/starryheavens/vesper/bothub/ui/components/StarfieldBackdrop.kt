package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

/**
 * Theme-adaptive spatial backdrop without distracting glow or GPU overdraw.
 */
@Composable
fun StarfieldBackdrop(modifier: Modifier = Modifier) {
    val theme = BotHubThemeTokens
    Canvas(modifier = modifier.fillMaxSize()) {
        drawRect(color = theme.bgBase)

        drawRect(
            brush = Brush.radialGradient(
                0.0f to theme.auroraPrimary,
                0.55f to theme.auroraPrimary.copy(alpha = theme.auroraPrimary.alpha * 0.35f),
                1.0f to Color.Transparent,
                center = Offset(size.width * 0.14f, size.height * 0.08f),
                radius = size.maxDimension * 0.55f
            )
        )

        drawRect(
            brush = Brush.radialGradient(
                0.0f to theme.auroraSecondary,
                0.50f to theme.auroraSecondary.copy(alpha = theme.auroraSecondary.alpha * 0.35f),
                1.0f to Color.Transparent,
                center = Offset(size.width * 0.88f, size.height * 0.84f),
                radius = size.maxDimension * 0.50f
            )
        )
    }
}
