package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

/**
 * Restrained obsidian-champagne spatial backdrop matching Web StarfieldBg atmosphere
 * without distracting glow or GPU overdraw.
 */
@Composable
fun StarfieldBackdrop(modifier: Modifier = Modifier) {
    Canvas(modifier = modifier.fillMaxSize()) {
        drawRect(color = Color(0xFF050811))

        drawRect(
            brush = Brush.radialGradient(
                0.0f to Color(0x1F6366F1),
                0.55f to Color(0x0A6366F1),
                1.0f to Color.Transparent,
                center = Offset(size.width * 0.14f, size.height * 0.08f),
                radius = size.maxDimension * 0.55f
            )
        )

        drawRect(
            brush = Brush.radialGradient(
                0.0f to Color(0x1AF2B880),
                0.50f to Color(0x08F2B880),
                1.0f to Color.Transparent,
                center = Offset(size.width * 0.88f, size.height * 0.84f),
                radius = size.maxDimension * 0.50f
            )
        )
    }
}
